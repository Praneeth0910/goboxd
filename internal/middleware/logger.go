package middleware

import (
	"log"
	"net/http"
	"time"
)

// ResponseWriter wraps http.ResponseWriter to capture status code
type ResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *ResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Logger is HTTP middleware that logs requests as JSON
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer
		wrappedWriter := &ResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call next handler
		next.ServeHTTP(wrappedWriter, r)

		// Log request
		duration := time.Since(start)
		log.Printf(
			`{"method": "%s", "path": "%s", "status": %d, "duration_ms": %d}`,
			r.Method,
			r.RequestURI,
			wrappedWriter.statusCode,
			duration.Milliseconds(),
		)
	})
}
