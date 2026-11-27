// Package middleware contains shared HTTP middleware.
package middleware

import "net/http"

const maxRequestBodyBytes = 1 << 20

// LimitBody bounds the size of an HTTP request body.
func LimitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		next.ServeHTTP(w, r)
	})
}
