package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/musyase/trace-weave/internal/trace"
)

func TestHTMLEscapesServiceAndOperation(t *testing.T) {
	start := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	span := trace.Span{TraceID: strings.Repeat("a", 32), SpanID: strings.Repeat("1", 16), Service: "<script>alert(1)</script>", Name: "<img src=x>", Start: start, End: start.Add(time.Second), Status: "ok"}
	var output bytes.Buffer
	if err := HTML(&output, trace.Analyze([]trace.Span{span})); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "<script>alert(1)</script>") || strings.Contains(output.String(), "<img src=x>") {
		t.Fatal("unescaped HTML")
	}
	if !strings.Contains(output.String(), "&lt;script&gt;") {
		t.Fatal("escaped text missing")
	}
}
