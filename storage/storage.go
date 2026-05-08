package storage

import (
	"context"
	"errors"
	"mime/multipart"
	"strings"
	"time"
)

var (
	ErrDiskNotFound = errors.New("storage disk not found")
	ErrInvalidPath  = errors.New("invalid storage path")
	ErrTooLarge     = errors.New("upload too large")
	ErrUnsupported  = errors.New("storage operation unsupported")
)

const (
	DiskAttachment = "attachment"
	DiskAvatar     = "avatar"
	DiskFiles      = "files"
)

type Config struct {
	DefaultDisk   string                `yaml:"default_disk"`
	MaxUploadSize int64                 `yaml:"max_upload_size"`
	Disks         map[string]DiskConfig `yaml:"disks"`
}

type DiskConfig struct {
	Backend       string       `yaml:"backend"`
	RootDir       string       `yaml:"root_dir"`
	PublicBaseURL string       `yaml:"public_base_url"`
	UpYun         UpYunConfig  `yaml:"upyun"`
	Local         LocalOptions `yaml:"local"`
}

type LocalOptions struct {
	CreateDirMode uint32 `yaml:"create_dir_mode"`
	FileMode      uint32 `yaml:"file_mode"`
}

type UpYunConfig struct {
	Bucket   string `yaml:"bucket"`
	Operator string `yaml:"operator"`
	Password string `yaml:"password"`
	UseHTTP  bool   `yaml:"use_http"`
}

type Storage interface {
	Get(ctx context.Context, path string) ([]byte, error)
	Put(ctx context.Context, path string, data []byte) error
	SaveUpload(ctx context.Context, file *multipart.FileHeader, options SaveOptions) (*ObjectInfo, error)
	SaveBytes(ctx context.Context, data []byte, contentType string, options SaveOptions) (*ObjectInfo, error)
	Delete(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
	URL(path string, options ...URLOption) string
	OpenPath(path string) (string, error)
}

type Disk = Storage

type Manager struct {
	disks       map[string]Disk
	defaultDisk string
}

type SaveOptions struct {
	Disk         string
	UID          string
	Dir          string
	Now          time.Time
	MaxBytes     int64
	Filename     string
	OriginalName string
	Overwrite    bool
}

type ObjectInfo struct {
	Disk        string `json:"disk"`
	Path        string `json:"path"`
	URL         string `json:"url"`
	Size        int64  `json:"size"`
	Checksum    string `json:"checksum"`
	ContentType string `json:"content_type"`
}

type URLOptions struct {
	Size string
}

type URLOption func(*URLOptions)

func WithSize(size string) URLOption {
	return func(options *URLOptions) {
		options.Size = size
	}
}

func NewManager(cfg Config) *Manager {
	disks := make(map[string]Disk, len(cfg.Disks))
	for name, diskCfg := range cfg.Disks {
		switch strings.ToLower(strings.TrimSpace(diskCfg.Backend)) {
		case "upyun":
			disks[name] = NewUpYunDisk(diskCfg, cfg.MaxUploadSize)
		case "external":
			disks[name] = NewExternalDisk(diskCfg)
		default:
			disks[name] = NewLocalDisk(diskCfg, cfg.MaxUploadSize)
		}
	}

	defaultDisk := cfg.DefaultDisk
	if defaultDisk == "" {
		if _, ok := disks["default"]; ok {
			defaultDisk = "default"
		} else if len(disks) == 1 {
			for name := range disks {
				defaultDisk = name
				break
			}
		}
	}
	if len(disks) == 0 {
		defaultDisk = "default"
		disks[defaultDisk] = NewLocalDisk(DiskConfig{
			RootDir:       "./storage",
			PublicBaseURL: "/storage/",
		}, cfg.MaxUploadSize)
	}
	return &Manager{
		disks:       disks,
		defaultDisk: defaultDisk,
	}
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (manager *Manager) Disk(name string) Disk {
	if manager == nil {
		return nil
	}
	if name == "" {
		name = manager.defaultDisk
	}
	return manager.disks[name]
}

func (manager *Manager) SaveUpload(ctx context.Context, file *multipart.FileHeader, options SaveOptions) (*ObjectInfo, error) {
	if manager == nil {
		return nil, ErrDiskNotFound
	}
	diskName := options.Disk
	if diskName == "" {
		diskName = manager.defaultDisk
	}
	disk := manager.Disk(diskName)
	if disk == nil {
		return nil, ErrDiskNotFound
	}
	info, err := disk.SaveUpload(ctx, file, options)
	if err != nil {
		return nil, err
	}
	info.Disk = diskName
	return info, nil
}

func (manager *Manager) Get(ctx context.Context, diskName string, objectPath string) ([]byte, error) {
	disk := manager.Disk(diskName)
	if disk == nil {
		return nil, ErrDiskNotFound
	}
	return disk.Get(ctx, objectPath)
}

func (manager *Manager) Put(ctx context.Context, diskName string, objectPath string, data []byte) error {
	disk := manager.Disk(diskName)
	if disk == nil {
		return ErrDiskNotFound
	}
	return disk.Put(ctx, objectPath, data)
}

func (manager *Manager) SaveBytes(ctx context.Context, data []byte, contentType string, options SaveOptions) (*ObjectInfo, error) {
	if manager == nil {
		return nil, ErrDiskNotFound
	}
	diskName := options.Disk
	if diskName == "" {
		diskName = manager.defaultDisk
	}
	disk := manager.Disk(diskName)
	if disk == nil {
		return nil, ErrDiskNotFound
	}
	info, err := disk.SaveBytes(ctx, data, contentType, options)
	if err != nil {
		return nil, err
	}
	info.Disk = diskName
	return info, nil
}

func (manager *Manager) Delete(ctx context.Context, diskName string, objectPath string) error {
	disk := manager.Disk(diskName)
	if disk == nil {
		return ErrDiskNotFound
	}
	return disk.Delete(ctx, objectPath)
}

func (manager *Manager) Exists(ctx context.Context, diskName string, objectPath string) (bool, error) {
	disk := manager.Disk(diskName)
	if disk == nil {
		return false, ErrDiskNotFound
	}
	return disk.Exists(ctx, objectPath)
}

func (manager *Manager) URL(diskName string, objectPath string, options ...URLOption) string {
	disk := manager.Disk(diskName)
	if disk == nil {
		return ""
	}
	return disk.URL(objectPath, options...)
}

func (manager *Manager) OpenPath(diskName string, objectPath string) (string, error) {
	disk := manager.Disk(diskName)
	if disk == nil {
		return "", ErrDiskNotFound
	}
	return disk.OpenPath(objectPath)
}

func (manager *Manager) OpenPublicPath(publicPath string) (string, error) {
	if manager == nil {
		return "", ErrDiskNotFound
	}
	cleaned, err := cleanObjectPath(publicPath)
	if err != nil {
		return "", err
	}
	diskName, objectPath, ok := splitDiskPath(cleaned)
	if ok {
		if disk := manager.Disk(diskName); disk != nil {
			return disk.OpenPath(objectPath)
		}
	}
	disk := manager.Disk("")
	if disk == nil {
		return "", ErrDiskNotFound
	}
	return disk.OpenPath(cleaned)
}

func publicURL(publicBaseURL string, objectPath string, urlOptions ...URLOption) string {
	cleaned, err := cleanObjectPath(objectPath)
	if err != nil {
		return ""
	}
	if publicBaseURL == "" {
		return ""
	}
	base := publicBaseURL
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	return appendURLOptions(base+strings.TrimPrefix(cleaned, "/"), urlOptions...)
}

func appendURLOptions(result string, urlOptions ...URLOption) string {
	options := URLOptions{}
	for _, option := range urlOptions {
		option(&options)
	}
	if options.Size != "" {
		result += "!" + options.Size
	}
	return result
}
