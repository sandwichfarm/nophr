package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/sandwichfarm/nophr/internal/config"
)

func TestRetentionMetadataMigration(t *testing.T) {
	// Create temporary database
	tmpFile := "/tmp/test_retention_migration.db"
	os.Remove(tmpFile)
	defer os.Remove(tmpFile)

	// Initialize storage with minimal config
	cfg := &config.Storage{
		Driver:     "sqlite",
		SQLitePath: tmpFile,
	}

	ctx := context.Background()
	st, err := New(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to initialize storage: %v", err)
	}
	defer st.Close()

	t.Log("✅ Storage initialized successfully")

	// Check if retention_metadata table exists
	db := st.DB()
	var tableName string
	err = db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='retention_metadata'
	`).Scan(&tableName)

	if err == sql.ErrNoRows {
		t.Fatal("❌ retention_metadata table NOT found")
	} else if err != nil {
		t.Fatalf("Failed to query table: %v", err)
	}

	t.Logf("✅ retention_metadata table exists: %s", tableName)

	// Check table schema
	rows, err := db.Query(`PRAGMA table_info(retention_metadata)`)
	if err != nil {
		t.Fatalf("Failed to get table info: %v", err)
	}
	defer rows.Close()

	expectedColumns := map[string]bool{
		"event_id":           false,
		"rule_name":          false,
		"rule_priority":      false,
		"retain_until":       false,
		"last_evaluated_at":  false,
		"score":              false,
		"protected":          false,
	}

	t.Log("📋 Table schema:")
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString

		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		if _, expected := expectedColumns[name]; expected {
			expectedColumns[name] = true
		}

		pkStr := ""
		if pk > 0 {
			pkStr = " [PRIMARY KEY]"
		}
		notnullStr := ""
		if notnull > 0 {
			notnullStr = " [NOT NULL]"
		}

		t.Logf("  - %s: %s%s%s", name, ctype, notnullStr, pkStr)
	}

	// Verify all expected columns exist
	for col, found := range expectedColumns {
		if !found {
			t.Errorf("Missing expected column: %s", col)
		}
	}

	// Check indexes
	indexRows, err := db.Query(`
		SELECT name FROM sqlite_master
		WHERE type='index' AND tbl_name='retention_metadata'
		AND name NOT LIKE 'sqlite_%'
	`)
	if err != nil {
		t.Fatalf("Failed to query indexes: %v", err)
	}
	defer indexRows.Close()

	t.Log("📑 Indexes:")
	indexCount := 0
	expectedIndexes := []string{
		"idx_retention_metadata_retain_until",
		"idx_retention_metadata_score",
		"idx_retention_metadata_protected",
	}
	foundIndexes := make(map[string]bool)

	for indexRows.Next() {
		var indexName string
		if err := indexRows.Scan(&indexName); err != nil {
			continue
		}
		t.Logf("  - %s", indexName)
		foundIndexes[indexName] = true
		indexCount++
	}

	// Verify expected indexes exist
	for _, idx := range expectedIndexes {
		if !foundIndexes[idx] {
			t.Errorf("Missing expected index: %s", idx)
		}
	}

	if indexCount == 0 {
		t.Error("No indexes found, expected 3")
	}

	t.Log("✅ Database migration test PASSED!")
}
