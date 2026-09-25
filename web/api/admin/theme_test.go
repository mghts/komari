package admin

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/komari-monitor/komari/database/models"
)

// TestIsValidThemeShort_PathTraversal 防止 DeleteTheme/UpdateTheme/SetTheme
// 的路径穿越漏洞：req.Short 直接进入 filepath.Join("./data/theme", short)，
// 若 short 含 "../" 会规范化到 ./data/theme 之外，配合 os.RemoveAll 可删除
// 工作目录外任意目录。isValidMarketShort 必须拒绝所有此类 payload。
func TestIsValidThemeShort_PathTraversal(t *testing.T) {
	// 必须被拒绝：路径穿越 / 绝对路径 / 目录分隔符 / 空值 / default
	deny := []string{
		"",
		"default",
		"..",
		"../",
		"./",
		"../../etc",
		"../..",
		"foo/../bar",
		"/etc/passwd",
		"foo/bar",
		"foo\\bar",
		"a b",
		"a;b",
		"a$(id)",
	}
	for _, in := range deny {
		if isValidMarketShort(in) {
			t.Errorf("isValidMarketShort(%q) = true, want false (路径穿越/非法字符未被拦截)", in)
		}
	}

	// 必须被接受：仅字母数字下划线连字符
	accept := []string{
		"mytheme",
		"my-theme",
		"my_theme",
		"theme123",
		"ABC",
		"a",
	}
	for _, in := range accept {
		if !isValidMarketShort(in) {
			t.Errorf("isValidMarketShort(%q) = false, want true (合法名称被误拒)", in)
		}
	}
}

func TestFailedThemeReplacementRetainsInstalledTheme(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "corrupt-theme.zip")
	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	for _, item := range []struct{ name, content string }{
		{"komari-theme.json", `{"name":"Test","short":"retain","version":"1.0.0"}`},
		{"asset.txt", "corrupt-me"},
	} {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: item.name, Method: zip.Store})
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
	archive, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	index := bytes.Index(archive, []byte("corrupt-me"))
	if index < 0 {
		t.Fatal("stored ZIP entry was not found")
	}
	archive[index] ^= 1
	if err := os.WriteFile(zipPath, archive, 0600); err != nil {
		t.Fatal(err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(workDir) })
	oldPath := filepath.Join("data", "theme", "retain", "old.txt")
	if err := os.MkdirAll(filepath.Dir(oldPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("old theme"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := extractAndValidateTheme(zipPath); err == nil {
		t.Fatal("corrupt theme update unexpectedly succeeded")
	}
	content, err := os.ReadFile(oldPath)
	if err != nil || string(content) != "old theme" {
		t.Fatalf("old theme was lost: content=%q err=%v", content, err)
	}
}

func TestValidateThemeManifestAcceptsLocalizedMetadata(t *testing.T) {
	theme := models.Theme{
		Name: map[string]any{
			"zh-CN": "配置字段演示主题",
			"en":    "Managed Configuration Demo Theme",
		},
		Short: "managed-config-demo",
		Description: map[string]any{
			"zh-CN": "用于验证全部托管配置字段的测试主题。",
			"en":    "A test theme covering every managed configuration field type.",
		},
		Author: map[string]any{
			"zh-CN": "Komari 团队",
			"en":    "Komari",
		},
	}

	if err := validateThemeManifest(theme); err != nil {
		t.Fatalf("localized theme metadata was rejected: %v", err)
	}
}

func TestPeekThemeFromZipAcceptsLocalizedMetadata(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "localized-theme.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(archive)
	manifest, err := writer.Create("komari-theme.json")
	if err != nil {
		t.Fatalf("create manifest: %v", err)
	}
	_, err = manifest.Write([]byte(`{
  "name": {"zh-CN": "本地化主题", "en": "Localized Theme"},
  "short": "localized-theme",
  "description": {"zh-CN": "描述", "en": "Description"},
  "author": {"zh-CN": "作者", "en": "Author"},
  "version": "1.0.0"
}`))
	if err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}

	theme, err := peekThemeFromZip(zipPath)
	if err != nil {
		t.Fatalf("localized theme package was rejected: %v", err)
	}
	if theme.Short != "localized-theme" {
		t.Fatalf("short = %q, want localized-theme", theme.Short)
	}

	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("change working directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(workDir)
	})

	installed, err := extractAndValidateTheme(zipPath)
	if err != nil {
		t.Fatalf("localized theme upload package was rejected: %v", err)
	}
	if installed.Short != "localized-theme" {
		t.Fatalf("installed short = %q, want localized-theme", installed.Short)
	}
}
