package apperrors

import "errors"

var (
	ErrBadRequest           = errors.New("bad request")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrForbidden            = errors.New("forbidden")
	ErrNotFound             = errors.New("not found")
	ErrConflict             = errors.New("conflict")
	ErrPayloadTooLarge      = errors.New("payload too large")
	ErrUnsupportedMediaType = errors.New("unsupported media type")
	ErrServiceUnavailable   = errors.New("service unavailable")
	ErrInternal             = errors.New("internal error")
)
