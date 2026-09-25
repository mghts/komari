package backup

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeTestArchive(t *testing.T, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backup.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	writer := zip.NewWriter(file)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close archive writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive file: %v", err)
	}
	return path
}

func TestValidateArchiveRejectsCorruptedEntry(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "corrupt.zip")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, item := range []struct{ name, content string }{
		{"komari-backup-markup", "marker"},
		{"komari.db", "corrupt-me"},
	} {
		header := &zip.FileHeader{Name: item.name, Method: zip.Store}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	index := bytes.Index(data, []byte("corrupt-me"))
	if index < 0 {
		t.Fatal("stored ZIP entry was not found")
	}
	data[index] ^= 1
	if err := os.WriteFile(archive, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ValidateArchive(archive); err == nil {
		t.Fatal("corrupt backup entry passed validation")
	}
}

func TestValidateArchiveAcceptsLegacyRootLayout(t *testing.T) {
	archive := writeTestArchive(t, map[string]string{
		"komari.db":            "database",
		"theme/config.json":    "{}",
		"komari-backup-markup": "backup marker",
	})
	if err := ValidateArchive(archive); err != nil {
		t.Fatalf("ValidateArchive rejected legacy root layout: %v", err)
	}
}

func TestValidateArchiveRequiresMarkup(t *testing.T) {
	archive := writeTestArchive(t, map[string]string{"komari.db": "database"})
	if err := ValidateArchive(archive); err == nil {
		t.Fatal("ValidateArchive accepted archive without markup")
	}
}

func TestValidateArchiveRequiresDatabase(t *testing.T) {
	archive := writeTestArchive(t, map[string]string{
		"komari-backup-markup": "backup marker",
		"theme/config.json":    "{}",
	})
	if err := ValidateArchive(archive); err == nil {
		t.Fatal("ValidateArchive accepted archive without komari.db")
	}
}
