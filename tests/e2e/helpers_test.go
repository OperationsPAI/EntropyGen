// Package e2e provides shared helpers for end-to-end tests.
package e2e

import (
	"io"
	"net/http"
)

// httpClient is the shared HTTP client for all e2e tests.
var httpClient = http.DefaultClient

// newHTTPRequest is a thin wrapper around http.NewRequest that panics on error
// (errors here are programmer mistakes, not runtime failures).
func newHTTPRequest(method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, url, body) //nolint:wrapcheck
}

// readAll reads all bytes from a reader, ignoring errors (used for response bodies in tests).
func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
