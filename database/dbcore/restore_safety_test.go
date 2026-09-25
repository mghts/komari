package dbcore

import (
	"archive/zip"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateStagedDatabaseAcceptsSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "komari.db")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY)"); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validateStagedDatabase(path); err != nil {
		t.Fatalf("valid SQLite backup was rejected: %v", err)
	}
}

func TestInvalidPendingBackupDoesNotRemoveCurrentData(t *testing.T) {
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })
	if err := os.MkdirAll("data", 0755); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join("data", "komari.db")
	if err := os.WriteFile(currentPath, []byte("current database"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("data", "backup.zip"), []byte("invalid archive"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := doInitialize(); err == nil {
		t.Fatal("invalid pending backup did not prevent startup")
	}
	content, err := os.ReadFile(currentPath)
	if err != nil || string(content) != "current database" {
		t.Fatalf("current data changed after rejected restore: content=%q err=%v", content, err)
	}
}

func TestInvalidSQLiteInPendingBackupDoesNotRemoveCurrentData(t *testing.T) {
	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })
	if err := os.MkdirAll("data", 0755); err != nil {
		t.Fatal(err)
	}
	currentPath := filepath.Join("data", "komari.db")
	if err := os.WriteFile(currentPath, []byte("current database"), 0600); err != nil {
		t.Fatal(err)
	}
	archive, err := os.Create(filepath.Join("data", "backup.zip"))
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for name, content := range map[string]string{
		"komari-backup-markup": "backup marker",
		"komari.db":            "invalid SQLite database",
	} {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := doInitialize(); err == nil {
		t.Fatal("invalid SQLite database did not prevent restore")
	}
	content, err := os.ReadFile(currentPath)
	if err != nil || string(content) != "current database" {
		t.Fatalf("current data changed after rejected restore: content=%q err=%v", content, err)
	}
}
