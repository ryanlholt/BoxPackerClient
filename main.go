// Command boxpackerclient is a thin client around the boxpacker library.
//
// It reads a packing problem as JSON and writes the solution as JSON, either
// over stdin/stdout (the default) or as an HTTP service.
//
//	# stdin/stdout
//	boxpackerclient < problem.json > solution.json
//
//	# HTTP service: POST the same JSON to /pack
//	boxpackerclient -http :8080
//	curl -s --data-binary @problem.json localhost:8080/pack
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	httpAddr := flag.String("http", "", "run as an HTTP service on this address (e.g. :8080) instead of reading stdin")
	pretty := flag.Bool("pretty", false, "pretty-print JSON output (stdin/stdout mode)")
	flag.Parse()

	if *httpAddr != "" {
		if err := serve(*httpAddr); err != nil {
			fmt.Fprintln(os.Stderr, "boxpackerclient:", err)
			os.Exit(1)
		}
		return
	}

	if err := runStdio(os.Stdin, os.Stdout, *pretty); err != nil {
		fmt.Fprintln(os.Stderr, "boxpackerclient:", err)
		os.Exit(1)
	}
}

// runStdio reads one JSON request from r and writes one JSON response to w.
func runStdio(r io.Reader, w io.Writer, pretty bool) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("parsing JSON input: %w", err)
	}

	resp, err := Pack(&req)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(resp)
}
