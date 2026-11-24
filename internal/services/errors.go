package services

import "errors"

var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden: insufficient permissions")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrValidation      = errors.New("validation error")
	ErrInvalidInput    = ErrValidation
	ErrConflict        = errors.New("conflict")
	ErrPayloadTooLarge = errors.New("payload too large")
	ErrSelfFollow      = errors.New("users cannot follow themselves")
	ErrAlreadyReady    = errors.New("asset is already marked as ready")
)
