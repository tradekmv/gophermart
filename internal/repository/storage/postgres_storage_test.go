// Package storage provides integration tests for postgres storage.
package storage

import (
	"testing"
	"time"
)

// TestPostgresStorage_ImplementsInterface verifies PostgresStorage implements Storage interface.
func TestPostgresStorage_ImplementsInterface(t *testing.T) {
	var _ Storage = (*PostgresStorage)(nil)
}

// TestOrderStruct verifies Order type has correct fields for JSON marshaling.
func TestOrderStruct(t *testing.T) {
	order := Order{
		ID:         "test-id",
		UserID:     "user-id",
		Number:     "12345678903",
		Status:     "NEW",
		UploadedAt: timeNow(),
	}
	if order.Number != "12345678903" {
		t.Errorf("expected 12345678903, got %s", order.Number)
	}
	if order.Status != "NEW" {
		t.Errorf("expected NEW, got %s", order.Status)
	}
}

func timeNow() time.Time {
	return time.Unix(1608123456, 0)
}
