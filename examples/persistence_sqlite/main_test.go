package main

import (
	"os"
	"testing"
)

func TestSQLitePersistenceExample(t *testing.T) {
	dbFile := "test_cache.db"
	if err := run(dbFile); err != nil {
		t.Fatalf("Example run failed: %v", err)
	}

	// Verify cleanup
	if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
		t.Errorf("Database file %s was not cleaned up", dbFile)
		os.Remove(dbFile)
	}
}
