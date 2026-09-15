package repository

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SQL generation coverage only: this does not claim to exercise an actual MySQL lock.
func TestArticleLockingReadDatabaseDialects(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dialect  gorm.Dialector
		wantLock bool
	}{
		{"mysql", mysql.New(mysql.Config{DSN: "test:test@tcp(127.0.0.1:3306)/test", SkipInitializeWithVersion: true}), true},
		{"sqlite", sqlite.Open(":memory:"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			db, err := gorm.Open(tc.dialect, &gorm.Config{
				DryRun: true, DisableAutomaticPing: true,
				Logger: logger.New(log.New(&output, "", 0), logger.Config{LogLevel: logger.Info}),
			})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			// DryRun leaves model fields empty; userID 0 permits inspecting generated SQL.
			if _, err := findOwnedArticleForUpdate(db, 0, 18); err != nil {
				t.Fatal(err)
			}
			query := output.String()
			if !strings.Contains(query, "SELECT") || !strings.Contains(query, "18") || strings.Contains(query, "FOR UPDATE") != tc.wantLock {
				t.Fatalf("unexpected locking query for %s: %s", tc.name, query)
			}
		})
	}
}
