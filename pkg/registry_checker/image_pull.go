package registry_checker

import (
	"errors"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

func IsAbsent(err error) bool {
	var transpErr *transport.Error
	errors.As(err, &transpErr)

	if transpErr == nil {
		return false
	}

	for _, transportError := range transpErr.Errors {
		if transportError.Code == transport.ManifestUnknownErrorCode {
			return true
		}
	}

	return false
}

func IsAuthnFail(err error) bool {
	var transpErr *transport.Error
	errors.As(err, &transpErr)

	if transpErr == nil {
		return false
	}

	for _, transportError := range transpErr.Errors {
		if transportError.Code == transport.UnauthorizedErrorCode {
			return true
		}
	}

	return false
}

func IsAuthzFail(err error) bool {
	var transpErr *transport.Error
	errors.As(err, &transpErr)

	if transpErr == nil {
		return false
	}

	for _, transportError := range transpErr.Errors {
		if transportError.Code == transport.DeniedErrorCode {
			return true
		}
	}

	return false
}

func IsOldRegistry(err error) bool {
	var schemaErr *remote.ErrSchema1
	errors.As(err, &schemaErr)

	if schemaErr != nil {
		return true
	}

	return false
}
