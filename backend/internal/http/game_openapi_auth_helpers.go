package http

import (
	"errors"
	"io"
	"net/http"
	"strings"

	gamesvc "game-admin/backend/internal/services/game"
	"github.com/gin-gonic/gin"
)

func gameOpenAPIAuthMiddleware(service *gamesvc.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if service == nil {
			respondError(c, http.StatusInternalServerError, errors.New("game service is nil"))
			c.Abort()
			return
		}
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			respondError(c, http.StatusBadRequest, err)
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(strings.NewReader(string(body)))
		result, err := service.AuthenticateOpenAPI(gamesvc.OpenAPIAuthInput{
			AccessKey:      c.GetHeader(gameAccessKeyHeader),
			TimestampValue: c.GetHeader(gameTimestampHeader),
			Nonce:          c.GetHeader(gameNonceHeader),
			Signature:      c.GetHeader(gameSignatureHeader),
			Method:         c.Request.Method,
			Path:           c.Request.URL.Path,
			RawQuery:       c.Request.URL.RawQuery,
			Body:           body,
		})
		if err != nil {
			status := http.StatusUnauthorized
			if strings.Contains(err.Error(), "not active") || strings.Contains(err.Error(), "expired") {
				status = http.StatusForbidden
			}
			respondError(c, status, err)
			c.Abort()
			return
		}
		c.Set(gameOpenAPIContextKey, gameOpenAPIContext{
			Credential: result.Credential,
			Game:       result.Game,
			Scopes:     stringSetFromJSON(result.Credential.Scopes),
		})
		c.Next()
	}
}

func gameOpenAPIContextFromGin(c *gin.Context) gameOpenAPIContext {
	if value, ok := c.Get(gameOpenAPIContextKey); ok {
		if ctx, ok := value.(gameOpenAPIContext); ok {
			return ctx
		}
	}
	return gameOpenAPIContext{}
}

func requireGameOpenAPIScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := gameOpenAPIContextFromGin(c)
		if _, ok := ctx.Scopes[scope]; !ok {
			respondError(c, http.StatusForbidden, errors.New("game openapi scope is not allowed"))
			c.Abort()
			return
		}
		c.Next()
	}
}
