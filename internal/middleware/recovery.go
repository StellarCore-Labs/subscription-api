package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// maxStackLogLength bounds how much of a stack trace is written to the logs.
const maxStackLogLength = 4000

// sanitizeStack truncates oversized stack traces before they reach the logs.
func sanitizeStack(stack string) string {
	if len(stack) <= maxStackLogLength {
		return stack
	}
	return stack[:maxStackLogLength] + "... (truncated)"
}

// ErrorResponse is the JSON body returned when a panic is recovered.
type ErrorResponse struct {
	Error   string    `json:"error"`
	Code    string    `json:"code"`
	Request string    `json:"request,omitempty"`
	Time    time.Time `json:"time"`
}

// Recovery converts panics into a structured 500 response, propagating the
// request ID to both the response header and body.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				requestID := requestIDForResponse(c)
				log.Printf("panic recovered request_id=%v err=%v stack=%s",
					requestID, rec, sanitizeStack(string(debug.Stack())))

				c.Header(RequestIDHeader, requestID)

				if c.Writer.Written() {
					// The response is already on the wire; the status cannot change.
					c.Abort()
					return
				}

				if wantsPlainText(c) {
					c.Header("Content-Type", "text/plain; charset=utf-8")
					c.AbortWithStatus(http.StatusInternalServerError)
					fmt.Fprintf(c.Writer, "Internal Server Error\nRequest ID: %s\n", requestID)
					return
				}

				c.Header("Content-Type", "application/json; charset=utf-8")
				c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
					Error:   "Internal server error",
					Code:    "INTERNAL_ERROR",
					Request: requestID,
					Time:    time.Now().UTC(),
				})
			}
		}()

		c.Next()
	}
}

// requestIDForResponse resolves the request ID from the context or the incoming
// header, generating a new one when neither carries a usable value.
func requestIDForResponse(c *gin.Context) string {
	if id, exists := c.Get(RequestIDKey); exists {
		if requestID, ok := id.(string); ok && requestID != "" {
			return requestID
		}
	}

	if incoming := sanitizeRequestID(c.GetHeader(RequestIDHeader)); incoming != "" {
		return incoming
	}

	return generateRequestID()
}

// wantsPlainText reports whether the caller asked for a text/plain response.
func wantsPlainText(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept"), "text/plain")
}
