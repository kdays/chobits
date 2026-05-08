package app

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestRunHTTPRequiresServer(t *testing.T) {
	err := RunHTTP(context.Background(), nil, Options{})
	if err == nil || !strings.Contains(err.Error(), "http server is nil") {
		t.Fatalf("expected nil server error, got %v", err)
	}
}

func TestRunHTTPReturnsListenErrorAndClosesResources(t *testing.T) {
	closer := &testCloser{}
	srv := &http.Server{Addr: "invalid address"}

	err := RunHTTP(context.Background(), srv, Options{Closers: []Closer{closer}})
	if err == nil {
		t.Fatalf("expected listen error")
	}
	if !closer.closed {
		t.Fatalf("expected closer to run after listen error")
	}
}

type testCloser struct {
	closed bool
}

func (closer *testCloser) Close() error {
	closer.closed = true
	return nil
}
