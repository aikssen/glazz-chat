package database

import (
	"testing"
	"testing/fstest"
)

func TestMigrationHashesAreDeterministic(t *testing.T) {
	files := fstest.MapFS{
		"00002_second.sql": {Data: []byte("SELECT 2;")},
		"00001_first.sql":  {Data: []byte("SELECT 1;")},
	}

	first, err := migrationHashes(files)
	if err != nil {
		t.Fatalf("migrationHashes() error = %v", err)
	}
	second, err := migrationHashes(files)
	if err != nil {
		t.Fatalf("migrationHashes() second error = %v", err)
	}
	if first[1] != second[1] || first[2] != second[2] {
		t.Fatalf("hashes differ: %v != %v", first, second)
	}
}

func TestMigrationHashesRejectInvalidFilename(t *testing.T) {
	files := fstest.MapFS{"invalid.sql": {Data: []byte("SELECT 1;")}}
	if _, err := migrationHashes(files); err == nil {
		t.Fatal("migrationHashes() error = nil")
	}
}
