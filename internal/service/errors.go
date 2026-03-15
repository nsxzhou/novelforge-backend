package service

import (
	"errors"
	"fmt"

	"inkmuse/backend/internal/infra/storage"
)

var (
	// ErrInvalidInput 表示输入参数或实体不合法。
	ErrInvalidInput = errors.New("service: invalid input")
	// ErrNotFound 表示请求的实体不存在。
	ErrNotFound = errors.New("service: not found")
	// ErrConflict 表示请求与当前状态冲突。
	ErrConflict = errors.New("service: conflict")
)

func WrapInvalidInput(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%w: %v", ErrInvalidInput, err)
}

func WrapNotFound(err error) error {
	if err == nil {
		return ErrNotFound
	}
	return fmt.Errorf("%w: %v", ErrNotFound, err)
}

func WrapConflict(err error) error {
	if err == nil {
		return ErrConflict
	}
	return fmt.Errorf("%w: %v", ErrConflict, err)
}

func TranslateStorageError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, storage.ErrNotFound):
		return WrapNotFound(err)
	case errors.Is(err, storage.ErrAlreadyExists):
		return WrapConflict(err)
	default:
		return err
	}
}
