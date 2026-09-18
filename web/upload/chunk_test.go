package upload

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredPluginUploadCannotStartOrResume(t *testing.T) {
	store := &Store{Root: t.TempDir(), MaxSize: 1024}
	if _, err := store.Init(Purpose("plugin"), "plugin.zip", 1); err == nil {
		t.Fatal("plugin upload accepted")
	}
	entries, err := os.ReadDir(store.Root)
	if err != nil || len(entries) != 0 {
		t.Fatalf("rejected upload created files: %v", err)
	}
	const id = "b1f996b8-bd3a-4ee7-ad96-4dde631ff18d"
	dir := filepath.Join(store.Root, id)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "upload.json"), []byte(`{"purpose":"plugin","size":1,"filename":"plugin.zip"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveChunk(id, 0, strings.NewReader("x")); err == nil {
		t.Fatal("retired upload resumed")
	}
	for _, purpose := range []Purpose{PurposeTheme, PurposeBackup} {
		if _, err := store.Init(purpose, "archive.zip", 1); err != nil {
			t.Fatalf("%s upload rejected: %v", purpose, err)
		}
	}
}
