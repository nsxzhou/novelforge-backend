package memory

import "errors"

var (
	ErrNotFound      = errors.New("memory storage: not found")
	ErrAlreadyExists = errors.New("memory storage: already exists")
)
