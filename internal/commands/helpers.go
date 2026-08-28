package commands

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gobijan/usetix-cli/internal/api"
	"github.com/gobijan/usetix-cli/internal/output"
	"github.com/gobijan/usetix-cli/internal/terminal"
)

const maxErrorMessageLength = 2048

func NormalizeError(err error) error {
	if err == nil {
		return nil
	}

	var structured *output.Error
	if errors.As(err, &structured) {
		return structured
	}

	var apiError *api.APIError
	if errors.As(err, &apiError) {
		message := sanitizeErrorMessage(apiError.Error())
		switch apiError.StatusCode {
		case http.StatusUnauthorized:
			return &output.Error{Code: "auth_required", Message: "Authentication failed", Hint: "Run: usetix auth login", HTTPStatus: apiError.StatusCode, Cause: err}
		case http.StatusForbidden:
			return &output.Error{Code: "forbidden", Message: message, HTTPStatus: apiError.StatusCode, Cause: err}
		case http.StatusNotFound:
			return &output.Error{Code: "not_found", Message: message, HTTPStatus: apiError.StatusCode, Cause: err}
		case http.StatusUnprocessableEntity:
			return &output.Error{Code: "validation", Message: message, HTTPStatus: apiError.StatusCode, Cause: err}
		case http.StatusTooManyRequests:
			rateLimit := output.ErrRateLimit(apiError.RetryAfter)
			rateLimit.Cause = err
			return rateLimit
		default:
			return &output.Error{Code: "api_error", Message: message, HTTPStatus: apiError.StatusCode, Retryable: apiError.StatusCode >= 500, Cause: err}
		}
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &output.Error{Code: "network", Message: "Request interrupted", Hint: sanitizeErrorMessage(err.Error()), Retryable: true, Cause: err}
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return output.ErrNetwork(errors.New(sanitizeErrorMessage(err.Error())))
	}

	return &output.Error{Code: "api_error", Message: sanitizeErrorMessage(err.Error()), Cause: err}
}

func sanitizeErrorMessage(message string) string {
	message = terminal.SanitizeLine(message)
	if len(message) > maxErrorMessageLength {
		return message[:maxErrorMessageLength] + "..."
	}
	return message
}

func normalizedMethod(value string) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(value))
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return method, nil
	default:
		return "", output.ErrUsage(fmt.Sprintf("unsupported HTTP method: %s", value))
	}
}

func summaryCount(count int, singular, plural string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}
