//go:build !upyun

package storage

import (
	"context"
	"mime/multipart"
)

type UpYunDisk struct {
	publicBaseURL string
}

func NewUpYunDisk(cfg DiskConfig, _ int64) *UpYunDisk {
	return &UpYunDisk{
		publicBaseURL: cfg.PublicBaseURL,
	}
}

func (disk *UpYunDisk) Get(context.Context, string) ([]byte, error) {
	return nil, ErrUnsupported
}

func (disk *UpYunDisk) Put(context.Context, string, []byte) error {
	return ErrUnsupported
}

func (disk *UpYunDisk) SaveUpload(context.Context, *multipart.FileHeader, SaveOptions) (*ObjectInfo, error) {
	return nil, ErrUnsupported
}

func (disk *UpYunDisk) SaveBytes(context.Context, []byte, string, SaveOptions) (*ObjectInfo, error) {
	return nil, ErrUnsupported
}

func (disk *UpYunDisk) Delete(context.Context, string) error {
	return ErrUnsupported
}

func (disk *UpYunDisk) Exists(context.Context, string) (bool, error) {
	return false, ErrUnsupported
}

func (disk *UpYunDisk) URL(objectPath string, options ...URLOption) string {
	return publicURL(disk.publicBaseURL, objectPath, options...)
}

func (disk *UpYunDisk) OpenPath(string) (string, error) {
	return "", ErrUnsupported
}
