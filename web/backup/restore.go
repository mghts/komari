// Package backup contains the shared upload preparation for backup restores.
package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"
)

var restoreMutex sync.Mutex

const (
	MaxArchiveSize    int64 = 4 << 30 // 4 GiB
	maxArchiveEntries       = 100_000
)

// RestoreLock serializes staging a backup until the caller releases it.
// A successful restore must hold this lock until the process restarts so a
// second request cannot replace backup.zip in the meantime.
type RestoreLock struct {
	once sync.Once
}

func AcquireRestoreLock() (*RestoreLock, error) {
	if !restoreMutex.TryLock() {
		return nil, fmt.Errorf("another restore operation is already in progress")
	}
	return &RestoreLock{}, nil
}

func (l *RestoreLock) Release() {
	l.once.Do(restoreMutex.Unlock)
}

// SaveUploadedBackup validates a Komari backup and stages it for restoration
// during the next process startup.
func SaveUploadedBackup(file io.Reader, filename string) error {
	lock, err := AcquireRestoreLock()
	if err != nil {
		return err
	}
	defer lock.Release()
	return lock.SaveUploadedBackup(file, filename)
}

// SaveUploadedBackup stages a backup while the caller holds the restore lock.
func (l *RestoreLock) SaveUploadedBackup(file io.Reader, filename string) error {
	if !strings.HasSuffix(strings.ToLower(filename), ".zip") {
		return fmt.Errorf("uploaded file must be a ZIP archive")
	}
	if err := os.MkdirAll("./data", 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Stage alongside backup.zip so the final rename is atomic on every
	// supported platform and never crosses filesystem boundaries.
	tempFile, err := os.CreateTemp("./data", ".backup-upload-*.zip")
	if err != nil {
		return fmt.Errorf("create temporary backup: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	written, err := io.Copy(tempFile, io.LimitReader(file, MaxArchiveSize+1))
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("save uploaded backup: %w", err)
	}
	if written > MaxArchiveSize {
		tempFile.Close()
		return fmt.Errorf("backup archive exceeds the %d byte limit", MaxArchiveSize)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close uploaded backup: %w", err)
	}

	if err := ValidateArchive(tempPath); err != nil {
		return err
	}

	finalPath := filepath.Join(".", "data", "backup.zip")
	if err := os.Rename(tempPath, finalPath); err != nil {
		return fmt.Errorf("publish backup file: %w", err)
	}
	return nil
}

// ValidateArchive checks the backup marker and bounds archive expansion before
// the startup restore path extracts it into data/.
func ValidateArchive(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open backup archive: %w", err)
	}
	defer reader.Close()

	if len(reader.File) > maxArchiveEntries {
		return fmt.Errorf("backup archive has too many files: %d", len(reader.File))
	}

	var expandedSize uint64
	var actualSize int64
	hasMarkup := false
	hasContent := false
	hasDatabase := false
	for _, entry := range reader.File {
		cleanName := pathpkg.Clean(entry.Name)
		if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, "../") || strings.HasPrefix(entry.Name, "/") {
			return fmt.Errorf("backup archive contains unsafe path: %s", entry.Name)
		}
		if cleanName == "backup.zip" || cleanName == "backup" || strings.HasPrefix(cleanName, "backup/") {
			return fmt.Errorf("backup archive contains reserved path: %s", entry.Name)
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup archive contains symlink: %s", entry.Name)
		}
		if entry.Name == "komari-backup-markup" {
			hasMarkup = true
		} else if entry.Name == "komari.db" && !entry.FileInfo().IsDir() {
			hasDatabase = true
			hasContent = true
		} else if !entry.FileInfo().IsDir() {
			hasContent = true
		}
		if entry.UncompressedSize64 > uint64(MaxArchiveSize) || expandedSize > uint64(MaxArchiveSize)-entry.UncompressedSize64 {
			return fmt.Errorf("backup archive expands beyond the %d byte limit", MaxArchiveSize)
		}
		expandedSize += entry.UncompressedSize64
		if entry.FileInfo().IsDir() {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open backup entry %s: %w", entry.Name, err)
		}
		remaining := MaxArchiveSize - actualSize
		written, copyErr := io.Copy(io.Discard, io.LimitReader(rc, remaining+1))
		closeErr := rc.Close()
		if copyErr != nil {
			return fmt.Errorf("verify backup entry %s: %w", entry.Name, copyErr)
		}
		if written > remaining {
			return fmt.Errorf("backup archive expands beyond the %d byte limit", MaxArchiveSize)
		}
		actualSize += written
		if closeErr != nil {
			return fmt.Errorf("close backup entry %s: %w", entry.Name, closeErr)
		}
	}
	if !hasMarkup {
		return fmt.Errorf("invalid backup file: missing komari-backup-markup file")
	}
	if !hasContent {
		return fmt.Errorf("invalid backup file: no data files")
	}
	if !hasDatabase {
		return fmt.Errorf("invalid backup file: missing komari.db")
	}
	return nil
}
