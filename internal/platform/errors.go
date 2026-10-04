package platform

import "errors"

var (
	ErrInvalid      = errors.New("invalid input")
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("already exists")
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("permission denied")
)
