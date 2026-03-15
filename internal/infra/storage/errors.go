package storage

import "inkmuse/backend/internal/infra/storage/shared"

var (
	// ErrNotFound 表示请求的实体不存在。
	ErrNotFound = shared.ErrNotFound
	// ErrAlreadyExists 表示实体的键(key)已存在。
	ErrAlreadyExists = shared.ErrAlreadyExists
)
