//go:build upyun

package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/upyun/go-sdk/v3/upyun"
)

type UpYunDisk struct {
	client        *upyun.UpYun
	publicBaseURL string
	maxUploadSize int64
}

func NewUpYunDisk(cfg DiskConfig, maxUploadSize int64) *UpYunDisk {
	return &UpYunDisk{
		client: upyun.NewUpYun(&upyun.UpYunConfig{
			Bucket:   cfg.UpYun.Bucket,
			Operator: cfg.UpYun.Operator,
			Password: cfg.UpYun.Password,
			UseHTTP:  cfg.UpYun.UseHTTP,
		}),
		publicBaseURL: cfg.PublicBaseURL,
		maxUploadSize: maxUploadSize,
	}
}

func (disk *UpYunDisk) Get(ctx context.Context, objectPath string) ([]byte, error) {
	ctx = ensureContext(ctx)
	cleaned, err := cleanObjectPath(objectPath)
	if err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	var buf bytes.Buffer
	_, err = disk.client.Get(&upyun.GetObjectConfig{
		Path:   "/" + strings.TrimPrefix(cleaned, "/"),
		Writer: &buf,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (disk *UpYunDisk) Put(ctx context.Context, objectPath string, data []byte) error {
	_, err := disk.SaveBytes(ctx, data, "", SaveOptions{
		Filename:  objectPath,
		Overwrite: true,
	})
	return err
}

func (disk *UpYunDisk) SaveUpload(ctx context.Context, fileHeader *multipart.FileHeader, options SaveOptions) (*ObjectInfo, error) {
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

func (disk *UpYunDisk) SaveBytes(ctx context.Context, data []byte, contentType string, options SaveOptions) (*ObjectInfo, error) {
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

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	checksum := sha256.Sum256(data)
	headers := map[string]string{
		"Content-Length": strconv.Itoa(len(data)),
		"Content-Type":   contentType,
	}
	if err := disk.client.Put(&upyun.PutObjectConfig{
		Path:    "/" + strings.TrimPrefix(objectPath, "/"),
		Reader:  bytes.NewReader(data),
		Headers: headers,
	}); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		_ = disk.Delete(context.Background(), objectPath)
		return nil, ctx.Err()
	default:
	}

	return &ObjectInfo{
		Path:        objectPath,
		URL:         disk.URL(objectPath),
		Size:        int64(len(data)),
		Checksum:    hex.EncodeToString(checksum[:]),
		ContentType: contentType,
	}, nil
}

func (disk *UpYunDisk) Delete(ctx context.Context, objectPath string) error {
	ctx = ensureContext(ctx)
	cleaned, err := cleanObjectPath(objectPath)
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	err = disk.client.Delete(&upyun.DeleteObjectConfig{Path: "/" + strings.TrimPrefix(cleaned, "/")})
	if upyun.IsNotExist(err) {
		return nil
	}
	return err
}

func (disk *UpYunDisk) Exists(ctx context.Context, objectPath string) (bool, error) {
	ctx = ensureContext(ctx)
	cleaned, err := cleanObjectPath(objectPath)
	if err != nil {
		return false, err
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}
	_, err = disk.client.GetInfo("/" + strings.TrimPrefix(cleaned, "/"))
	if err == nil {
		return true, nil
	}
	if upyun.IsNotExist(err) || strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "file not exists") {
		return false, nil
	}
	return false, err
}

func (disk *UpYunDisk) URL(objectPath string, options ...URLOption) string {
	return publicURL(disk.publicBaseURL, objectPath, options...)
}

func (disk *UpYunDisk) OpenPath(string) (string, error) {
	return "", ErrUnsupported
}
