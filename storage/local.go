package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"time"
)

type LocalDisk struct {
	rootDir       string
	publicBaseURL string
	maxUploadSize int64
	dirMode       os.FileMode
	fileMode      os.FileMode
}

func NewLocalDisk(cfg DiskConfig, maxUploadSize int64) *LocalDisk {
	dirMode := os.FileMode(cfg.Local.CreateDirMode)
	if dirMode == 0 {
		dirMode = 0o755
	}
	fileMode := os.FileMode(cfg.Local.FileMode)
	if fileMode == 0 {
		fileMode = 0o644
	}
	return &LocalDisk{
		rootDir:       cfg.RootDir,
		publicBaseURL: cfg.PublicBaseURL,
		maxUploadSize: maxUploadSize,
		dirMode:       dirMode,
		fileMode:      fileMode,
	}
}

func (disk *LocalDisk) Get(ctx context.Context, objectPath string) ([]byte, error) {
	ctx = ensureContext(ctx)
	cleaned, err := cleanObjectPath(objectPath)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(disk.rootDir, filepath.FromSlash(cleaned)))
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return data, nil
}

func (disk *LocalDisk) Put(ctx context.Context, objectPath string, data []byte) error {
	_, err := disk.SaveBytes(ctx, data, "", SaveOptions{
		Filename:  objectPath,
		Overwrite: true,
	})
	return err
}

func (disk *LocalDisk) SaveUpload(ctx context.Context, fileHeader *multipart.FileHeader, options SaveOptions) (*ObjectInfo, error) {
	ctx = ensureContext(ctx)
	if fileHeader == nil {
		return nil, ErrInvalidPath
	}
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = disk.maxUploadSize
	}
	if maxBytes > 0 && fileHeader.Size > maxBytes {
		return nil, ErrTooLarge
	}

	source, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer source.Close()

	limited := io.Reader(source)
	if maxBytes > 0 {
		limited = io.LimitReader(source, maxBytes+1)
	}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, ErrTooLarge
	}

	contentType := fileHeader.Header.Get("Content-Type")
	if options.OriginalName == "" {
		options.OriginalName = fileHeader.Filename
	}
	return disk.SaveBytes(ctx, data, contentType, options)
}

func (disk *LocalDisk) SaveBytes(ctx context.Context, data []byte, contentType string, options SaveOptions) (*ObjectInfo, error) {
	ctx = ensureContext(ctx)
	maxBytes := options.MaxBytes
	if maxBytes <= 0 {
		maxBytes = disk.maxUploadSize
	}
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		return nil, ErrTooLarge
	}

	now := options.Now
	if now.IsZero() {
		now = time.Now()
	}
	objectPath := options.Filename
	var err error
	if objectPath == "" {
		originalName := options.OriginalName
		if originalName == "" {
			originalName = options.Filename
		}
		objectPath, err = makeObjectPath(now, options.UID, originalName)
		if err != nil {
			return nil, err
		}
	}
	if options.Dir != "" {
		objectPath = path.Join(options.Dir, objectPath)
	}
	objectPath, err = cleanObjectPath(objectPath)
	if err != nil {
		return nil, err
	}
	if contentType == "" {
		contentType = http.DetectContentType(data[:min(len(data), 512)])
	}

	targetPath := filepath.Join(disk.rootDir, filepath.FromSlash(objectPath))
	if err := os.MkdirAll(filepath.Dir(targetPath), disk.dirMode); err != nil {
		return nil, err
	}

	flags := os.O_CREATE | os.O_WRONLY
	if options.Overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	target, err := os.OpenFile(targetPath, flags, disk.fileMode)
	if err != nil {
		return nil, err
	}
	defer target.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(target, hasher)
	if _, err := io.Copy(writer, bytes.NewReader(data)); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		_ = os.Remove(targetPath)
		return nil, ctx.Err()
	default:
	}

	return &ObjectInfo{
		Path:        objectPath,
		URL:         disk.URL(objectPath),
		Size:        int64(len(data)),
		Checksum:    hex.EncodeToString(hasher.Sum(nil)),
		ContentType: contentType,
	}, nil
}

func (disk *LocalDisk) Delete(_ context.Context, objectPath string) error {
	cleaned, err := cleanObjectPath(objectPath)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(disk.rootDir, filepath.FromSlash(cleaned)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (disk *LocalDisk) Exists(_ context.Context, objectPath string) (bool, error) {
	cleaned, err := cleanObjectPath(objectPath)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(filepath.Join(disk.rootDir, filepath.FromSlash(cleaned)))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func (disk *LocalDisk) URL(objectPath string, _ ...URLOption) string {
	return publicURL(disk.publicBaseURL, objectPath)
}

func (disk *LocalDisk) OpenPath(objectPath string) (string, error) {
	cleaned, err := cleanObjectPath(objectPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(disk.rootDir, filepath.FromSlash(cleaned)), nil
}
