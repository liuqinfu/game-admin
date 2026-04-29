package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func authAccountFromContext(c *gin.Context) (authAccount, bool) {
	value, ok := c.Get("authAccount")
	if !ok {
		return authAccount{}, false
	}
	account, ok := value.(authAccount)
	return account, ok
}

func respondError(c *gin.Context, status int, err error) {
	code, message := errorCodeAndMessage(status, err)
	writeJSON(c, status, gin.H{
		"code":    code,
		"message": message,
		"error":   message,
	})
}

func errorCodeAndMessage(status int, err error) (string, string) {
	message := "internal server error"
	if err != nil {
		message = err.Error()
	}

	switch status {
	case http.StatusBadRequest:
		return "bad_request", message
	case http.StatusUnauthorized:
		return "unauthorized", message
	case http.StatusForbidden:
		return "forbidden", message
	case http.StatusNotFound:
		return "not_found", message
	case http.StatusConflict:
		return "conflict", message
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity", message
	case http.StatusTooManyRequests:
		return "too_many_requests", message
	default:
		if status >= 500 {
			return "internal_error", message
		}
		return "error", message
	}
}
