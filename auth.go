package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// Environment variables that configure HTTP authentication.
const (
	envAPIToken = "BOXPACKER_API_TOKEN" // required bearer token
	envCFSecret = "BOXPACKER_CF_SECRET" // optional Cloudflare header secret
)

// cfHeader is the request header Cloudflare is configured to inject (via a
// Transform Rule) so the origin can reject traffic that bypasses Cloudflare.
const cfHeader = "X-Origin-Auth"

const bearerPrefix = "Bearer "

// authConfig holds the secrets used to guard the HTTP service. The bearer token
// is always required; the Cloudflare header secret is enforced only when set.
type authConfig struct {
	bearerToken string // required; requests must send "Authorization: Bearer <token>"
	cfSecret    string // optional; when set, requests must also send cfHeader
}

// authFromEnv builds the auth config from the environment, failing closed if the
// required bearer token is missing.
func authFromEnv() (authConfig, error) {
	token := os.Getenv(envAPIToken)
	if token == "" {
		return authConfig{}, fmt.Errorf("%s must be set to run the HTTP service", envAPIToken)
	}
	return authConfig{
		bearerToken: token,
		cfSecret:    os.Getenv(envCFSecret),
	}, nil
}

// cloudflareEnforced reports whether the Cloudflare header check is active.
func (c authConfig) cloudflareEnforced() bool { return c.cfSecret != "" }

// requireAuth wraps next, rejecting any request that fails authentication. The
// bearer token is always checked; the Cloudflare header is checked only when a
// secret is configured.
func (c authConfig) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, bearerPrefix) ||
			!secretEqual(strings.TrimPrefix(auth, bearerPrefix), c.bearerToken) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if c.cloudflareEnforced() && !secretEqual(r.Header.Get(cfHeader), c.cfSecret) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// secretEqual reports whether a and b are equal, compared in constant time to
// avoid leaking the secret through response timing.
func secretEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
