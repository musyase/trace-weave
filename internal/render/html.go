package render

import (
	"fmt"
	"html/template"
	"io"
	"math"
	"time"

	"github.com/musyase/trace-weave/internal/trace"
)

type spanView struct {
	Service  string
	Name     string
	ID       string
	Parent   string
	Status   string
	Duration string
	Start    string
	End      string
	Left     float64
	Width    float64
}

type traceView struct {
	ID       string
	Duration string
	Start    string
	End      string
	Spans    []spanView
	Warnings []trace.Warning
	Errors   int
}

type pageView struct {
	Traces       []traceView
	SpanCount    int
	WarningCount int
	ErrorCount   int
	Generated    string
}

func HTML(writer io.Writer, traces []trace.Trace) error {
	page := pageView{Traces: make([]traceView, 0, len(traces)), Generated: time.Now().UTC().Format(time.RFC3339)}
	for _, item := range traces {
		view := traceView{
			ID: item.ID, Duration: item.Duration().String(),
			Start:    item.Start.UTC().Format(time.RFC3339Nano),
			End:      item.End.UTC().Format(time.RFC3339Nano),
			Spans:    make([]spanView, 0, len(item.Spans)),
			Warnings: item.Warnings, Errors: item.Errors,
		}
		window := math.Max(float64(item.Duration()), 1)
		for _, span := range item.Spans {
			left := math.Max(0, 100*float64(span.Start.Sub(item.Start))/window)
			width := math.Max(0.5, 100*float64(span.End.Sub(span.Start))/window)
			width = math.Max(0, math.Min(width, 100-left))
			left = math.Round(left*100) / 100
			width = math.Round(width*100) / 100
			view.Spans = append(view.Spans, spanView{
				Service: span.Service, Name: span.Name, ID: span.SpanID,
				Parent: span.ParentID, Status: span.Status,
				Duration: span.End.Sub(span.Start).String(),
				Start:    span.Start.UTC().Format(time.RFC3339Nano),
				End:      span.End.UTC().Format(time.RFC3339Nano),
				Left:     left, Width: width,
			})
		}
		page.SpanCount += len(item.Spans)
		page.WarningCount += len(item.Warnings)
		page.ErrorCount += item.Errors
		page.Traces = append(page.Traces, view)
	}
	if err := reportTemplate.Execute(writer, page); err != nil {
		return fmt.Errorf("render report: %w", err)
	}
	return nil
}

var reportTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Trace Weave report</title>
<style>
:root{font-family:ui-sans-serif,system-ui,-apple-system,Segoe UI,sans-serif;color:#182521;background:#f7f7f2;font-synthesis:none}
*{box-sizing:border-box}body{margin:0}.shell{max-width:1160px;margin:0 auto;padding:40px 28px 80px}
.eyebrow{font-size:12px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:#356c64}
h1{font-size:clamp(38px,5vw,62px);line-height:1.05;letter-spacing:-.04em;margin:12px 0 14px}
.intro{max-width:690px;color:#51625c;line-height:1.55;margin:0 0 30px}
.stats{display:grid;grid-template-columns:repeat(4,1fr);gap:10px;margin:0 0 36px}.stat{background:#e9eee9;border-radius:14px;padding:18px}.stat strong{display:block;font-size:32px;line-height:1.1}.stat span{font-size:13px;color:#52615b}
.trace{border:1px solid #cfd9d2;background:white;border-radius:18px;margin:18px 0;overflow:hidden}.trace summary{cursor:pointer;list-style:none;padding:20px 24px;display:flex;gap:16px;align-items:center;justify-content:space-between}.trace summary::-webkit-details-marker{display:none}.trace summary:focus-visible{outline:3px solid #1a655b;outline-offset:-4px}
.trace-title{min-width:0}.trace-title b{display:block;font-size:18px}.trace-title code{display:block;overflow-wrap:anywhere;font-size:12px;color:#63736d;margin-top:5px}.trace-meta{display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end}.pill{border-radius:100px;padding:6px 10px;background:#edf1ed;color:#31433c;font-size:12px;white-space:nowrap}.pill.error{background:#ffe4d3;color:#793f16}.pill.warning{background:#fff0bc;color:#63500d}
.trace-body{border-top:1px solid #e3e8e2;padding:20px 24px 26px}.timeframe{font-size:12px;color:#62716b;margin:0 0 14px;overflow-wrap:anywhere}.span-grid{display:grid;grid-template-columns:minmax(190px,290px) 1fr 72px;gap:10px 16px;align-items:center}.axis{grid-column:2;display:flex;justify-content:space-between;font-size:11px;color:#6a7770}.span-label{min-width:0}.span-label b{display:block;font-size:13px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.span-label small{display:block;color:#62716b;font-size:11px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.track{height:27px;background:repeating-linear-gradient(90deg,#edf1ed,#edf1ed calc(25% - 1px),#d6ded8 25%);border-radius:5px;position:relative}.bar{height:13px;position:absolute;top:7px;background:#2f8174;border-radius:3px;min-width:2px}.bar.error{background:#bd5832}.duration{text-align:right;font-size:12px;font-variant-numeric:tabular-nums}.warnings{margin-top:24px;padding-top:16px;border-top:1px solid #e3e8e2}.warnings h3{font-size:13px;margin:0 0 8px}.warnings ul{margin:0;padding-left:20px;color:#604f28;font-size:13px;line-height:1.6}.warnings code{font-size:11px}.foot{margin-top:36px;font-size:12px;color:#6b7972}
@media(max-width:700px){.shell{padding:24px 16px 60px}.stats{grid-template-columns:repeat(2,1fr)}.trace summary{align-items:flex-start;flex-direction:column}.trace-meta{justify-content:flex-start}.span-grid{grid-template-columns:minmax(100px,35%) 1fr;gap:9px}.duration{display:none}.trace-body{padding:18px 14px 24px}.span-label b{font-size:12px}}
@media(prefers-color-scheme:dark){:root{background:#151d1a;color:#e6eee8}.intro,.stat span,.trace-title code,.timeframe,.span-label small,.axis,.foot{color:#b5c5ba}.stat{background:#25342c}.trace{background:#1c2821;border-color:#405247}.trace-body,.warnings{border-color:#405247}.pill{background:#31443a;color:#dfebe1}.pill.error{background:#593321;color:#ffcfb5}.pill.warning{background:#514625;color:#ffeca8}.track{background:repeating-linear-gradient(90deg,#27362d,#27362d calc(25% - 1px),#4b5c4f 25%)}.warnings ul{color:#e2cd91}}
</style>
</head>
<body><main class="shell">
<div class="eyebrow">Local trace inspection</div>
<h1>Trace Weave</h1>
<p class="intro">A time-ordered view of spans from JSONL. Service names, operation names, timing, and parent links are shown. Other input fields are ignored. This file opens without a server.</p>
<section class="stats" aria-label="Report summary">
<div class="stat"><strong>{{len .Traces}}</strong><span>traces</span></div>
<div class="stat"><strong>{{.SpanCount}}</strong><span>spans</span></div>
<div class="stat"><strong>{{.ErrorCount}}</strong><span>error spans</span></div>
<div class="stat"><strong>{{.WarningCount}}</strong><span>structure warnings</span></div>
</section>
{{range .Traces}}<details class="trace" open>
<summary><span class="trace-title"><b>{{.Duration}} trace</b><code>{{.ID}}</code></span><span class="trace-meta"><span class="pill">{{len .Spans}} spans</span>{{if .Errors}}<span class="pill error">{{.Errors}} errors</span>{{end}}{{if .Warnings}}<span class="pill warning">{{len .Warnings}} warnings</span>{{end}}</span></summary>
<div class="trace-body"><p class="timeframe">{{.Start}} to {{.End}} UTC</p>
<div class="span-grid"><div></div><div class="axis"><span>Start</span><span>25%</span><span>50%</span><span>75%</span><span>End</span></div><div></div>
{{range .Spans}}<div class="span-label" title="{{.Service}} / {{.Name}} / {{.ID}}"><b>{{.Name}}</b><small>{{.Service}} · {{.ID}}</small></div><div class="track" title="{{.Start}} to {{.End}}"><div class="bar {{.Status}}" style="left:{{.Left}}%;width:{{.Width}}%"></div></div><div class="duration">{{.Duration}}</div>{{end}}</div>
{{if .Warnings}}<div class="warnings"><h3>Structural warnings</h3><ul>{{range .Warnings}}<li><b>{{.Code}}</b>{{if .SpanID}} on <code>{{.SpanID}}</code>{{end}}: {{.Detail}}</li>{{end}}</ul></div>{{end}}
</div></details>{{end}}
<p class="foot">Generated {{.Generated}}. Local report. Timing warnings are clues to inspect, not proof of a clock fault.</p>
</main></body></html>`))
