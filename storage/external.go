package storage

import (
	"context"
	"mime/multipart"
	"strings"
)

// ExternalDisk is a read-only disk that only turns stored paths into public URLs.
type ExternalDisk struct {
	publicBaseURL string
}

func NewExternalDisk(cfg DiskConfig) *ExternalDisk {
	return &ExternalDisk{publicBaseURL: cfg.PublicBaseURL}
}

func (disk *ExternalDisk) Get(context.Context, string) ([]byte, error) {
	return nil, ErrUnsupported
}

func (disk *ExternalDisk) Put(context.Context, string, []byte) error {
	return ErrUnsupported
}

func (disk *ExternalDisk) SaveUpload(context.Context, *multipart.FileHeader, SaveOptions) (*ObjectInfo, error) {
	return nil, ErrUnsupported
}

func (disk *ExternalDisk) SaveBytes(context.Context, []byte, string, SaveOptions) (*ObjectInfo, error) {
	return nil, ErrUnsupported
}

func (disk *ExternalDisk) Delete(context.Context, string) error {
	return ErrUnsupported
}

func (disk *ExternalDisk) Exists(context.Context, string) (bool, error) {
	return false, ErrUnsupported
}

func (disk *ExternalDisk) URL(objectPath string, options ...URLOption) string {
	value := strings.TrimSpace(objectPath)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(value, "//") {
		return appendURLOptions(value, options...)
	}
	if disk.publicBaseURL == "" {
		return appendURLOptions(value, options...)
	}
	return publicURL(disk.publicBaseURL, value, options...)
}

func (disk *ExternalDisk) OpenPath(string) (string, error) {
	return "", ErrUnsupported
}
