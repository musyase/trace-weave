package trace

import (
	"strings"
	"testing"
)

const valid = `{"trace_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","span_id":"1111111111111111","service":"web","name":"GET /","start":"2026-11-18T10:00:00Z","end":"2026-11-18T10:00:01Z","status":"ok"}`

func TestReadAndAnalyze(t *testing.T) {
	spans, err := ReadJSONL(strings.NewReader("\n" + valid + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	traces := Analyze(spans)
	if len(traces) != 1 || traces[0].Duration().Seconds() != 1 || len(traces[0].Warnings) != 0 {
		t.Fatalf("unexpected trace: %+v", traces)
	}
}

func TestInvalidRowsShowLineNumber(t *testing.T) {
	_, err := ReadJSONL(strings.NewReader(valid + "\n" + `{"trace_id":"bad"}`))
	if err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("wrong error: %v", err)
	}
}

func TestMissingParentAndOutsideWindow(t *testing.T) {
	input := valid + "\n" + `{"trace_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","span_id":"2222222222222222","parent_span_id":"3333333333333333","service":"db","name":"query","start":"2026-11-18T10:00:00Z","end":"2026-11-18T10:00:01Z"}` + "\n" + `{"trace_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","span_id":"4444444444444444","parent_span_id":"1111111111111111","service":"cache","name":"write","start":"2026-11-18T10:00:00Z","end":"2026-11-18T10:00:02Z"}`
	spans, err := ReadJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	warnings := Analyze(spans)[0].Warnings
	if len(warnings) != 2 || warnings[0].Code != "missing_parent" || warnings[1].Code != "outside_parent" {
		t.Fatalf("warnings: %+v", warnings)
	}
}

func TestCycleAndDuplicate(t *testing.T) {
	input := `{"trace_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","span_id":"1111111111111111","parent_span_id":"2222222222222222","service":"a","name":"one","start":"2026-11-18T10:00:00Z","end":"2026-11-18T10:00:01Z"}` + "\n" + `{"trace_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","span_id":"2222222222222222","parent_span_id":"1111111111111111","service":"b","name":"two","start":"2026-11-18T10:00:00Z","end":"2026-11-18T10:00:01Z"}` + "\n" + `{"trace_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","span_id":"2222222222222222","service":"b","name":"duplicate","start":"2026-11-18T10:00:00Z","end":"2026-11-18T10:00:01Z"}`
	spans, err := ReadJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, warning := range Analyze(spans)[0].Warnings {
		seen[warning.Code] = true
	}
	if !seen["duplicate_span"] || !seen["parent_cycle"] {
		t.Fatalf("missing warnings: %+v", seen)
	}
}

func TestUnknownFieldsAreNotRetained(t *testing.T) {
	input := strings.TrimSuffix(valid, "}") + `,"secret":"should not appear"}`
	spans, err := ReadJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(spans[0].Name, "secret") {
		t.Fatal("extra field leaked")
	}
}

func TestNegativeDurationIsReportedWithoutNegativeTraceLength(t *testing.T) {
	input := `{"trace_id":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","span_id":"1111111111111111","service":"web","name":"broken","start":"2026-11-18T10:00:02Z","end":"2026-11-18T10:00:01Z"}`
	spans, err := ReadJSONL(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	item := Analyze(spans)[0]
	if item.Duration() < 0 || len(item.Warnings) != 1 || item.Warnings[0].Code != "negative_duration" {
		t.Fatalf("trace: %+v", item)
	}
}

func TestRejectsControlCharactersInTerminalLabels(t *testing.T) {
	input := strings.Replace(valid, `"GET /"`, `"GET \n/"`, 1)
	_, err := ReadJSONL(strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "control characters") {
		t.Fatalf("wrong error: %v", err)
	}
}
