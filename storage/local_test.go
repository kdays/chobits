package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestManagerSaveBytesOpenURLAndDelete(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	manager := NewManager(Config{
		DefaultDisk:   "media",
		MaxUploadSize: 1024,
		Disks: map[string]DiskConfig{
			"media": {
				RootDir:       root,
				PublicBaseURL: "https://cdn.example.test/files",
			},
		},
	})

	info, err := manager.SaveBytes(ctx, []byte("hello"), "text/plain", SaveOptions{
		UID:          "u42",
		OriginalName: "Avatar.PNG",
		Now:          time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("save bytes: %v", err)
	}
	if info.Disk != "media" {
		t.Fatalf("unexpected disk: %q", info.Disk)
	}
	if !strings.HasPrefix(info.Path, "202605/u42_") || !strings.HasSuffix(info.Path, ".png") {
		t.Fatalf("unexpected generated path: %q", info.Path)
	}
	if info.Size != int64(len("hello")) {
		t.Fatalf("unexpected size: %d", info.Size)
	}
	if info.ContentType != "text/plain" {
		t.Fatalf("unexpected content type: %q", info.ContentType)
	}
	if want := "https://cdn.example.test/files/" + info.Path; info.URL != want {
		t.Fatalf("unexpected url: got %q want %q", info.URL, want)
	}
	if got := manager.URL("", info.Path, WithSize("small")); got != info.URL {
		t.Fatalf("local storage should ignore resized url options: got %q want %q", got, info.URL)
	}

	localPath, err := manager.OpenPath("", info.Path)
	if err != nil {
		t.Fatalf("open path: %v", err)
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected file data: %q", data)
	}
	readBack, err := manager.Get(ctx, "", info.Path)
	if err != nil {
		t.Fatalf("get saved object: %v", err)
	}
	if string(readBack) != "hello" {
		t.Fatalf("unexpected get data: %q", readBack)
	}
	exists, err := manager.Exists(ctx, "", info.Path)
	if err != nil {
		t.Fatalf("exists saved object: %v", err)
	}
	if !exists {
		t.Fatalf("expected saved object to exist")
	}

	if err := manager.Delete(ctx, "", info.Path); err != nil {
		t.Fatalf("delete saved file: %v", err)
	}
	if _, err := os.Stat(localPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to be deleted, got %v", err)
	}
}

func TestLocalDiskRejectsInvalidPathAndTooLarge(t *testing.T) {
	ctx := context.Background()
	disk := NewLocalDisk(DiskConfig{RootDir: t.TempDir()}, 3)

	if _, err := disk.SaveBytes(ctx, []byte("toolong"), "", SaveOptions{}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected too large error, got %v", err)
	}
	if _, err := disk.SaveBytes(ctx, []byte("ok"), "", SaveOptions{Filename: "../escape.txt"}); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected invalid path error, got %v", err)
	}
	if _, err := disk.OpenPath("../escape.txt"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("expected open path invalid path error, got %v", err)
	}
}

func TestManagerOpenPublicPathUsesDiskPrefix(t *testing.T) {
	avatarRoot := t.TempDir()
	manager := NewManager(Config{
		DefaultDisk: DiskAttachment,
		Disks: map[string]DiskConfig{
			DiskAttachment: {RootDir: t.TempDir()},
			DiskAvatar:     {RootDir: avatarRoot},
		},
	})

	got, err := manager.OpenPublicPath("avatar/202605/u.png")
	if err != nil {
		t.Fatalf("open public path: %v", err)
	}
	if want := filepath.Join(avatarRoot, "202605", "u.png"); got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestManagerUsesOnlyDiskAsDefault(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	manager := NewManager(Config{
		Disks: map[string]DiskConfig{
			"media": {RootDir: root},
		},
	})

	info, err := manager.SaveBytes(ctx, []byte("ok"), "text/plain", SaveOptions{Filename: "ok.txt"})
	if err != nil {
		t.Fatalf("save with only disk default: %v", err)
	}
	if info.Disk != "media" {
		t.Fatalf("disk = %q, want media", info.Disk)
	}
}

func TestManagerDoesNotGuessDefaultWhenMultipleDisks(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(Config{
		Disks: map[string]DiskConfig{
			"avatar":     {RootDir: t.TempDir()},
			"attachment": {RootDir: t.TempDir()},
		},
	})

	if _, err := manager.SaveBytes(ctx, []byte("ok"), "text/plain", SaveOptions{Filename: "ok.txt"}); !errors.Is(err, ErrDiskNotFound) {
		t.Fatalf("save without default disk error = %v, want ErrDiskNotFound", err)
	}
}

func TestManagerSupportsExplicitFilenameAndDiskLookup(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	manager := NewManager(Config{
		DefaultDisk: "private",
		Disks: map[string]DiskConfig{
			"private": {RootDir: root},
		},
	})

	info, err := manager.SaveBytes(ctx, []byte("report"), "", SaveOptions{
		Dir:       "reports",
		Filename:  "today.txt",
		Overwrite: true,
	})
	if err != nil {
		t.Fatalf("save explicit file: %v", err)
	}
	if info.Path != "reports/today.txt" {
		t.Fatalf("unexpected explicit path: %q", info.Path)
	}
	if _, err := os.Stat(filepath.Join(root, "reports", "today.txt")); err != nil {
		t.Fatalf("expected explicit file to exist: %v", err)
	}
	if err := manager.Put(ctx, "private", "reports/today.txt", []byte("updated")); err != nil {
		t.Fatalf("put explicit file: %v", err)
	}
	data, err := manager.Get(ctx, "private", "reports/today.txt")
	if err != nil {
		t.Fatalf("get explicit file: %v", err)
	}
	if string(data) != "updated" {
		t.Fatalf("unexpected updated data: %q", data)
	}
	if manager.Disk("missing") != nil {
		t.Fatalf("expected missing disk to be nil")
	}
	if _, err := manager.OpenPath("missing", info.Path); !errors.Is(err, ErrDiskNotFound) {
		t.Fatalf("expected missing disk error, got %v", err)
	}
}

func TestExternalDiskBuildsURLAndRejectsWrites(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(Config{
		DefaultDisk: DiskAvatar,
		Disks: map[string]DiskConfig{
			DiskAvatar: {
				Backend:       "external",
				PublicBaseURL: "https://avatar.example.test/base",
			},
		},
	})

	if got, want := manager.URL(DiskAvatar, "/u/01.png", WithSize("s")), "https://avatar.example.test/base/u/01.png!s"; got != want {
		t.Fatalf("external URL = %q, want %q", got, want)
	}
	if got, want := manager.URL(DiskAvatar, "https://cdn.example.test/a.png", WithSize("s")), "https://cdn.example.test/a.png!s"; got != want {
		t.Fatalf("absolute external URL = %q, want %q", got, want)
	}
	if _, err := manager.SaveBytes(ctx, []byte("x"), "text/plain", SaveOptions{Disk: DiskAvatar, Filename: "x.txt"}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected unsupported save, got %v", err)
	}
	if err := manager.Delete(ctx, DiskAvatar, "x.txt"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected unsupported delete, got %v", err)
	}
}

func TestUpYunDiskURLSupportsSizeOption(t *testing.T) {
	manager := NewManager(Config{
		DefaultDisk: DiskAttachment,
		Disks: map[string]DiskConfig{
			DiskAttachment: {
				Backend:       "upyun",
				PublicBaseURL: "https://img.example.test/covers",
			},
		},
	})

	got := manager.URL(DiskAttachment, "202605/cover.png", WithSize("y"))
	want := "https://img.example.test/covers/202605/cover.png!y"
	if got != want {
		t.Fatalf("upyun URL = %q, want %q", got, want)
	}
}
