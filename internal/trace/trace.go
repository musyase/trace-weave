package trace

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

const MaxSpans = 50000

var traceIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var spanIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

type Span struct {
	TraceID  string    `json:"trace_id"`
	SpanID   string    `json:"span_id"`
	ParentID string    `json:"parent_span_id,omitempty"`
	Service  string    `json:"service"`
	Name     string    `json:"name"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Status   string    `json:"status"`
}

type Warning struct {
	Code   string `json:"code"`
	SpanID string `json:"span_id,omitempty"`
	Detail string `json:"detail"`
}

type Trace struct {
	ID       string    `json:"trace_id"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Spans    []Span    `json:"spans"`
	Warnings []Warning `json:"warnings"`
	Errors   int       `json:"errors"`
}

func (t Trace) Duration() time.Duration { return t.End.Sub(t.Start) }

type inputSpan struct {
	TraceID  string `json:"trace_id"`
	SpanID   string `json:"span_id"`
	ParentID string `json:"parent_span_id"`
	Service  string `json:"service"`
	Name     string `json:"name"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Status   string `json:"status"`
}

// ReadJSONL accepts one span per line. Unknown fields are ignored so logs can
// carry attributes without copying them into the report.
func ReadJSONL(reader io.Reader) ([]Span, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	spans := make([]Span, 0)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if len(spans) == MaxSpans {
			return nil, fmt.Errorf("line %d: limit of %d spans exceeded", line, MaxSpans)
		}
		var raw inputSpan
		if err := json.Unmarshal([]byte(text), &raw); err != nil {
			return nil, fmt.Errorf("line %d: invalid JSON: %w", line, err)
		}
		span, err := validate(raw)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		spans = append(spans, span)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read JSONL: %w", err)
	}
	if len(spans) == 0 {
		return nil, fmt.Errorf("input contains no spans")
	}
	return spans, nil
}

func validate(raw inputSpan) (Span, error) {
	traceID := strings.ToLower(strings.TrimSpace(raw.TraceID))
	spanID := strings.ToLower(strings.TrimSpace(raw.SpanID))
	parentID := strings.ToLower(strings.TrimSpace(raw.ParentID))
	if !traceIDPattern.MatchString(traceID) {
		return Span{}, fmt.Errorf("trace_id must be 32 hexadecimal characters")
	}
	if !spanIDPattern.MatchString(spanID) {
		return Span{}, fmt.Errorf("span_id must be 16 hexadecimal characters")
	}
	if parentID != "" && !spanIDPattern.MatchString(parentID) {
		return Span{}, fmt.Errorf("parent_span_id must be 16 hexadecimal characters")
	}
	service := strings.TrimSpace(raw.Service)
	name := strings.TrimSpace(raw.Name)
	if service == "" || name == "" || len(service) > 120 || len(name) > 160 {
		return Span{}, fmt.Errorf("service and name are required and must be at most 120 and 160 bytes")
	}
	if strings.IndexFunc(service, unicode.IsControl) >= 0 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Span{}, fmt.Errorf("service and name cannot contain control characters")
	}
	start, err := time.Parse(time.RFC3339Nano, raw.Start)
	if err != nil {
		return Span{}, fmt.Errorf("start must be RFC3339")
	}
	end, err := time.Parse(time.RFC3339Nano, raw.End)
	if err != nil {
		return Span{}, fmt.Errorf("end must be RFC3339")
	}
	status := strings.ToLower(strings.TrimSpace(raw.Status))
	if status == "" {
		status = "unset"
	}
	if status != "ok" && status != "error" && status != "unset" {
		return Span{}, fmt.Errorf("status must be ok, error, or unset")
	}
	return Span{traceID, spanID, parentID, service, name, start, end, status}, nil
}

func Analyze(spans []Span) []Trace {
	groups := make(map[string][]Span)
	for _, span := range spans {
		groups[span.TraceID] = append(groups[span.TraceID], span)
	}
	result := make([]Trace, 0, len(groups))
	for id, group := range groups {
		sort.SliceStable(group, func(i, j int) bool {
			if group[i].Start.Equal(group[j].Start) {
				return group[i].SpanID < group[j].SpanID
			}
			return group[i].Start.Before(group[j].Start)
		})
		t := Trace{ID: id, Start: group[0].Start, End: group[0].End, Spans: group, Warnings: []Warning{}}
		if t.End.Before(t.Start) {
			t.End = t.Start
		}
		byID := make(map[string]Span)
		for _, span := range group {
			if span.Start.Before(t.Start) {
				t.Start = span.Start
			}
			if span.End.After(t.End) {
				t.End = span.End
			}
			if span.Start.After(t.End) {
				t.End = span.Start
			}
			if span.Status == "error" {
				t.Errors++
			}
			if _, exists := byID[span.SpanID]; exists {
				t.Warnings = append(t.Warnings, Warning{"duplicate_span", span.SpanID, "span ID appears more than once"})
			} else {
				byID[span.SpanID] = span
			}
			if span.End.Before(span.Start) {
				t.Warnings = append(t.Warnings, Warning{"negative_duration", span.SpanID, "end precedes start"})
			}
		}
		roots := 0
		for _, span := range group {
			if span.ParentID == "" {
				roots++
				continue
			}
			parent, exists := byID[span.ParentID]
			if !exists {
				t.Warnings = append(t.Warnings, Warning{"missing_parent", span.SpanID, "parent " + span.ParentID + " is absent"})
				continue
			}
			if span.Start.Before(parent.Start) || span.End.After(parent.End) {
				t.Warnings = append(t.Warnings, Warning{"outside_parent", span.SpanID, "span falls outside its parent's time range"})
			}
		}
		if roots == 0 {
			t.Warnings = append(t.Warnings, Warning{"no_root", "", "trace has no root span"})
		}
		if roots > 1 {
			t.Warnings = append(t.Warnings, Warning{"multiple_roots", "", fmt.Sprintf("trace has %d root spans", roots)})
		}
		visited := make(map[string]bool)
		for _, span := range group {
			if visited[span.SpanID] {
				continue
			}
			path := make(map[string]bool)
			current := span.SpanID
			for current != "" {
				if path[current] {
					t.Warnings = append(t.Warnings, Warning{"parent_cycle", current, "parent chain contains a cycle"})
					break
				}
				if visited[current] {
					break
				}
				path[current] = true
				next, exists := byID[current]
				if !exists {
					break
				}
				current = next.ParentID
			}
			for key := range path {
				visited[key] = true
			}
		}
		sort.Slice(t.Warnings, func(i, j int) bool {
			if t.Warnings[i].Code == t.Warnings[j].Code {
				return t.Warnings[i].SpanID < t.Warnings[j].SpanID
			}
			return t.Warnings[i].Code < t.Warnings[j].Code
		})
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Start.Equal(result[j].Start) {
			return result[i].ID < result[j].ID
		}
		return result[i].Start.Before(result[j].Start)
	})
	return result
}
