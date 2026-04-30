package client

import (
	"errors"
	"net/http"

	"github.com/nirvana-labs/nirvana-go/nks"
)

func IsNotFound(err error) bool {
	var apiErr *nks.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusNotFound
	}
	return false
}

func IsAvailabilityRejection(err error) bool {
	var apiErr *nks.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 400 && apiErr.StatusCode < 500
	}
	return false
}
