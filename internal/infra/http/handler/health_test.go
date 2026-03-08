package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type stubReadinessChecker struct {
	err error
}

func (s stubReadinessChecker) CheckReadiness(context.Context) error {
	return s.err
}

func TestReadyzReturnsOKWithoutChecker(t *testing.T) {
	h := server.Default()
	handler := NewHealthHandler(nil)
	h.GET("/readyz", handler.Readyz)

	resp := ut.PerformRequest(h.Engine, consts.MethodGet, "/readyz", nil)
	if resp.Code != consts.StatusOK {
		t.Fatalf("status code = %d, want %d", resp.Code, consts.StatusOK)
	}
}

func TestReadyzReturnsServiceUnavailableOnReadinessFailure(t *testing.T) {
	h := server.Default()
	handler := NewHealthHandler(stubReadinessChecker{err: errors.New("db unavailable")})
	h.GET("/readyz", handler.Readyz)

	resp := ut.PerformRequest(h.Engine, consts.MethodGet, "/readyz", nil)
	if resp.Code != consts.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", resp.Code, consts.StatusServiceUnavailable)
	}
}
