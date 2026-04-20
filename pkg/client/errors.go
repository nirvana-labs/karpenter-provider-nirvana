package client

import (
	"errors"
	"net/http"

	"github.com/nirvana-labs/nirvana-go/nks"
)

// IsNotFound reports whether the error represents a 404 (not found) response
// from the Nirvana API, indicating the resource has already been deleted.
func IsNotFound(err error) bool {
	var apiErr *nks.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

// IsAvailabilityRejection reports whether the error represents a client-side
// rejection (4xx) from the Nirvana API — typically a preflight capacity,
// quota, or validation failure. Transport errors and 5xx responses return false
// so callers can treat them as retryable rather than as capacity signals.
func IsAvailabilityRejection(err error) bool {
	var apiErr *nks.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
	}
	return false
}
