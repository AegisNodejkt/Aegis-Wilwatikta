package store

import "errors"

// Common sentinel errors for the store layer.
var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("store: entity not found")

	// ErrDuplicateEvent indicates a webhook event with the same idempotency key exists.
	ErrDuplicateEvent = errors.New("store: duplicate webhook event")

	// ErrInvalidTransition indicates an invalid state transition was attempted.
	ErrInvalidTransition = errors.New("store: invalid job state transition")
)
