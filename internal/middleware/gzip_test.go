package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompressMiddleware_NoAcceptEncoding(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler := CompressMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("plain text"))
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Header().Get("Content-Encoding") == "gzip" {
		t.Error("should not have gzip encoding")
	}
}

func TestCompressMiddleware_WithAcceptEncoding(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()

	handler := CompressMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("compressed content"))
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Error("should have gzip encoding")
	}

	// Verify content is actually gzip compressed
	reader, err := gzip.NewReader(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to decompress: %v", err)
	}
	if string(decompressed) != "compressed content" {
		t.Errorf("expected 'compressed content', got '%s'", string(decompressed))
	}
}

func TestCompressMiddleware_WriteHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rr := httptest.NewRecorder()

	handler := CompressMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, rr.Code)
	}
}

func TestGzipResponseWriter_Write(t *testing.T) {
	_ = httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	gz := gzip.NewWriter(rr)
	gw := &gzipResponseWriter{
		Writer: gz,
		w:      rr,
	}

	n, err := gw.Write([]byte("test data"))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if n != 9 {
		t.Errorf("expected 9 bytes written, got %d", n)
	}
	gz.Close()
}

func TestGzipResponseWriter_WriteHeader(t *testing.T) {
	_ = httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	gz := gzip.NewWriter(rr)
	gw := &gzipResponseWriter{
		Writer: gz,
		w:      rr,
	}

	gw.WriteHeader(http.StatusOK)
	if rr.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, rr.Code)
	}
	gz.Close()
}

func TestGzipResponseWriter_Header(t *testing.T) {
	_ = httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	gz := gzip.NewWriter(rr)
	gw := &gzipResponseWriter{
		Writer: gz,
		w:      rr,
	}

	h := gw.Header()
	h.Set("X-Header", "test")

	if rr.Header().Get("X-Header") != "test" {
		t.Error("expected header to be set")
	}
	gz.Close()
}

func TestDecompressBody_NoGzip(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("plain"))
	req.Header.Set("Content-Encoding", "")

	data, err := DecompressBody(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if data != nil {
		t.Error("expected nil for non-gzip")
	}
}

func TestDecompressBody_WithGzip(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	gz.Write([]byte("compressed data"))
	gz.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Encoding", "gzip")

	data, err := DecompressBody(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(data) != "compressed data" {
		t.Errorf("expected 'compressed data', got '%s'", string(data))
	}
}

func TestDecompressBody_InvalidGzip(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("not gzip"))
	req.Header.Set("Content-Encoding", "gzip")

	_, err := DecompressBody(req)
	if err == nil {
		t.Error("expected error for invalid gzip")
	}
}
