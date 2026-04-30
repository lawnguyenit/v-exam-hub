package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultImportRoot = "data/imports"

func ImportRoot() string {
	if value := strings.TrimSpace(os.Getenv("IMPORT_STORAGE_DIR")); value != "" {
		return value
	}
	return defaultImportRoot
}

func SaveImportFile(batchID int64, name string, content []byte) (string, error) {
	return saveImportObject(batchID, nil, name, content)
}

func SaveImportAsset(batchID int64, name string, content []byte) (string, error) {
	return saveImportObject(batchID, []string{"assets"}, name, content)
}

func RemoveImportBatch(batchID int64) error {
	return os.RemoveAll(filepath.Join(ImportRoot(), fmt.Sprintf("%d", batchID)))
}

func ResolveImportFile(storedPath string) (string, bool) {
	cleanPath := filepath.Clean(filepath.FromSlash(storedPath))
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", false
	}
	importRoot, err := filepath.Abs(ImportRoot())
	if err != nil {
		return "", false
	}
	if absPath != importRoot && !strings.HasPrefix(absPath, importRoot+string(os.PathSeparator)) {
		return "", false
	}
	return absPath, true
}

func saveImportObject(batchID int64, subdirs []string, name string, content []byte) (string, error) {
	parts := []string{ImportRoot(), fmt.Sprintf("%d", batchID)}
	parts = append(parts, subdirs...)
	dir := filepath.Join(parts...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(path), nil
}
