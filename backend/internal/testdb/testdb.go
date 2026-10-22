// Package testdb provides isolated schemas on a real PostgreSQL test database.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/reelcanon/reelcanon/backend/internal/database"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func Open(t *testing.T) *sql.DB {
	t.Helper()
	connection := os.Getenv("TEST_DATABASE_URL")
	if connection == "" {
		t.Skip("TEST_DATABASE_URL is required for PostgreSQL integration tests")
	}
	parsed, err := url.Parse(connection)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(strings.Trim(parsed.Path, "/"), "_test") {
		t.Fatal("integration database name must end with _test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := database.Open(ctx, connection)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	values := parsed.Query()
	values.Set("search_path", schema)
	parsed.RawQuery = values.Encode()
	db, err := database.Open(ctx, parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Close()
		admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		admin.Close()
	})
	_, file, _, _ := runtime.Caller(0)
	if err := database.ApplyMigrations(ctx, db, filepath.Join(filepath.Dir(file), "../../../database/migrations")); err != nil {
		t.Fatal(err)
	}
	return db
}
