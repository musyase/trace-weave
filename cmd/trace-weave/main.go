package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/musyase/trace-weave/internal/render"
	"github.com/musyase/trace-weave/internal/trace"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "trace-weave:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	flags := flag.NewFlagSet("trace-weave", flag.ContinueOnError)
	input := flags.String("input", "-", "JSONL file, or - for standard input")
	output := flags.String("output", "-", "output file, or - for standard output")
	format := flags.String("format", "text", "text, json, or html")
	filter := flags.String("trace", "", "one trace ID to show")
	flags.SetOutput(io.Discard)
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("%w (use -input, -output, -format, or -trace)", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *format != "text" && *format != "json" && *format != "html" {
		return fmt.Errorf("format must be text, json, or html")
	}
	var source io.Reader = stdin
	if *input != "-" {
		file, err := os.Open(*input)
		if err != nil {
			return err
		}
		defer file.Close()
		source = file
	}
	spans, err := trace.ReadJSONL(source)
	if err != nil {
		return err
	}
	traces := trace.Analyze(spans)
	if *filter != "" {
		chosen := make([]trace.Trace, 0, 1)
		for _, item := range traces {
			if item.ID == strings.ToLower(*filter) {
				chosen = append(chosen, item)
			}
		}
		if len(chosen) == 0 {
			return fmt.Errorf("trace %s was not found", *filter)
		}
		traces = chosen
	}
	var destination io.Writer = stdout
	if *output != "-" {
		file, err := os.Create(*output)
		if err != nil {
			return err
		}
		defer file.Close()
		destination = file
	}
	switch *format {
	case "json":
		encoder := json.NewEncoder(destination)
		encoder.SetIndent("", "  ")
		return encoder.Encode(traces)
	case "html":
		return render.HTML(destination, traces)
	default:
		for _, item := range traces {
			if _, err := fmt.Fprintf(destination, "%s  %s  %d spans  %d errors  %d warnings\n", item.ID, item.Duration(), len(item.Spans), item.Errors, len(item.Warnings)); err != nil {
				return err
			}
			for _, span := range item.Spans {
				if _, err := fmt.Fprintf(destination, "  %-16s %-28s %9s  %s\n", span.Service, span.Name, span.End.Sub(span.Start), span.Status); err != nil {
					return err
				}
			}
			for _, warning := range item.Warnings {
				if _, err := fmt.Fprintf(destination, "  ! %s %s: %s\n", warning.Code, warning.SpanID, warning.Detail); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
