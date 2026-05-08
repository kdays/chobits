package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"":        slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for value, want := range tests {
		got, err := ParseLevel(value)
		if err != nil {
			t.Fatalf("parse level %q: %v", value, err)
		}
		if got != want {
			t.Fatalf("parse level %q = %v, want %v", value, got, want)
		}
	}

	if _, err := ParseLevel("verbose"); err == nil {
		t.Fatalf("expected invalid level error")
	}
}

func TestNewChoosesFormat(t *testing.T) {
	var jsonOutput bytes.Buffer
	jsonLogger, err := New(&jsonOutput, Options{Level: "info"})
	if err != nil {
		t.Fatalf("new json logger: %v", err)
	}
	jsonLogger.Info("hello")
	if got := jsonOutput.String(); !strings.HasPrefix(got, "{") {
		t.Fatalf("expected json output, got %q", got)
	}

	var textOutput bytes.Buffer
	textLogger, err := New(&textOutput, Options{Env: "dev"})
	if err != nil {
		t.Fatalf("new text logger: %v", err)
	}
	textLogger.Info("hello")
	if got := textOutput.String(); strings.HasPrefix(got, "{") || !strings.Contains(got, "msg=hello") {
		t.Fatalf("expected text output, got %q", got)
	}
}
