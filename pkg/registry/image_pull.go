package registry

import (
	"errors"
	"net/http"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func IsAbsent(err error) bool {
	var transpErr *transport.Error
	errors.As(err, &transpErr)

	if transpErr == nil {
		return false
	}

	return transpErr.StatusCode == http.StatusNotFound
}

func IsAuthnFail(err error) bool {
	var transpErr *transport.Error
	errors.As(err, &transpErr)

	if transpErr == nil {
		return false
	}

	return transpErr.StatusCode == http.StatusUnauthorized
}

func IsAuthzFail(err error) bool {
	var transpErr *transport.Error
	errors.As(err, &transpErr)

	if transpErr == nil {
		return false
	}

	return transpErr.StatusCode == http.StatusForbidden
}

func IsOldRegistry(err error) bool {
	return errors.Is(err, remote.ErrSchema1)
}
