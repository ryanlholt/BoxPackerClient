package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// serve starts the HTTP service and blocks until it fails.
func serve(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/pack", handlePack)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	fmt.Fprintf(os.Stderr, "boxpackerclient: listening on %s (POST /pack)\n", addr)
	return http.ListenAndServe(addr, mux)
}

// handlePack packs the JSON body of a POST request and writes the JSON result.
func handlePack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "use POST")
		return
	}

	var req Request
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "parsing JSON input: "+err.Error())
		return
	}

	resp, err := Pack(&req)
	if err != nil {
		// Validation error: the request itself was malformed.
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Response{Error: msg})
}
