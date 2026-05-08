package storage

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func makeObjectPath(now time.Time, uid string, originalName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(originalName))
	if ext == "" {
		ext = ".bin"
	}

	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}

	prefix := uid
	if prefix == "" {
		prefix = "0"
	}
	filename := fmt.Sprintf("%s_%s%s", prefix, hex.EncodeToString(random), ext)
	return path.Join(now.Format("200601"), filename), nil
}

func cleanObjectPath(value string) (string, error) {
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "/")
	cleaned := path.Clean(value)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", ErrInvalidPath
	}
	return cleaned, nil
}

func splitDiskPath(value string) (string, string, bool) {
	diskName, objectPath, found := strings.Cut(value, "/")
	if !found || diskName == "" || objectPath == "" {
		return "", "", false
	}
	return diskName, objectPath, true
}
