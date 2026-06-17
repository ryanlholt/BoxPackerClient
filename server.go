package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Body-size cap for POST /pack. The default is generous: payloads stay small
// because item quantities are expressed as a field, not by repeating items.
const (
	envMaxBodyBytes     = "BOXPACKER_MAX_BODY_BYTES"
	defaultMaxBodyBytes = 10 << 20 // 10 MiB
)

// HTTP server timeouts, to protect against slow or stuck connections.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
)

// serve starts the HTTP service and blocks until it fails. It refuses to start
// unless authentication is configured (see authFromEnv).
func serve(addr string) error {
	auth, err := authFromEnv()
	if err != nil {
		return err
	}
	maxBody, err := maxBodyFromEnv()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	// /pack is guarded and body-capped; /healthz is intentionally left open for
	// liveness probes.
	mux.Handle("/pack", auth.requireAuth(limitBody(maxBody, http.HandlerFunc(handlePack))))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	cf := "disabled"
	if auth.cloudflareEnforced() {
		cf = "required (" + cfHeader + ")"
	}
	fmt.Fprintf(os.Stderr, "boxpackerclient: listening on %s (POST /pack, bearer token required, Cloudflare header %s, max body %d bytes)\n", addr, cf, maxBody)
	return srv.ListenAndServe()
}

// maxBodyFromEnv reads the POST /pack body-size cap from the environment,
// falling back to the default and failing on an invalid override.
func maxBodyFromEnv() (int64, error) {
	v := os.Getenv(envMaxBodyBytes)
	if v == "" {
		return defaultMaxBodyBytes, nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer (bytes), got %q", envMaxBodyBytes, v)
	}
	return n, nil
}

// limitBody caps the request body at maxBytes. Reads past the cap fail with a
// *http.MaxBytesError, which handlePack reports as 413.
func limitBody(maxBytes int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
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
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("request body exceeds %d bytes", maxErr.Limit))
			return
		}
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
