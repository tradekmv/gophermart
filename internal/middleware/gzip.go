// Package middleware provides HTTP middleware for the gophermart service.
package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// CompressMiddleware provides gzip compression for responses.
func CompressMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if client accepts gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Create gzip writer
		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer gz.Close()

		// Set header
		w.Header().Set("Content-Encoding", "gzip")

		// Wrap response writer
		gzWriter := &gzipResponseWriter{
			Writer: gz,
			w:      w,
		}

		next.ServeHTTP(gzWriter, r)
	})
}

type gzipResponseWriter struct {
	io.Writer
	w http.ResponseWriter
}

func (gw *gzipResponseWriter) Write(b []byte) (int, error) {
	return gw.Writer.Write(b)
}

func (gw *gzipResponseWriter) WriteHeader(statusCode int) {
	gw.w.WriteHeader(statusCode)
}

func (gw *gzipResponseWriter) Header() http.Header {
	return gw.w.Header()
}

// DecompressBody reads and decompresses gzipped request body.
func DecompressBody(r *http.Request) ([]byte, error) {
	if r.Header.Get("Content-Encoding") != "gzip" {
		return nil, nil
	}

	gz, err := gzip.NewReader(r.Body)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return io.ReadAll(gz)
}
