package handler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	appservice "novelforge/backend/internal/service"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type errorResponse struct {
	Error string `json:"error"`
}

func writeError(ctx *app.RequestContext, statusCode int, message string) {
	ctx.JSON(statusCode, errorResponse{Error: message})
}

func writeServiceError(ctx *app.RequestContext, err error) {
	switch {
	case errors.Is(err, appservice.ErrInvalidInput):
		writeError(ctx, consts.StatusBadRequest, err.Error())
	case errors.Is(err, appservice.ErrNotFound):
		writeError(ctx, consts.StatusNotFound, err.Error())
	case errors.Is(err, appservice.ErrConflict):
		writeError(ctx, consts.StatusConflict, err.Error())
	default:
		writeError(ctx, consts.StatusInternalServerError, "internal server error")
	}
}

func parseNonNegativeIntQuery(ctx *app.RequestContext, name string) (int, error) {
	value, exists := ctx.URI().QueryArgs().PeekExists(name)
	if !exists {
		return 0, nil
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("%s must not be empty", name)
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}

	return parsed, nil
}

func parseOptionalStringQuery(ctx *app.RequestContext, name string) (string, bool, error) {
	value, exists := ctx.URI().QueryArgs().PeekExists(name)
	if !exists {
		return "", false, nil
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", true, fmt.Errorf("%s must not be empty", name)
	}

	return value, true, nil
}
