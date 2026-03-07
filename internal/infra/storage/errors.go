package storage

import "errors"

var (
	// ErrNotFound 表示请求的实体不存在。
	ErrNotFound = errors.New("storage: not found")
	// ErrAlreadyExists 表示实体的键(key)已存在。
	ErrAlreadyExists = errors.New("storage: already exists")
)
