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
