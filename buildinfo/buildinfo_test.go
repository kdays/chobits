package buildinfo

import (
	"strings"
	"testing"
)

func TestTextAndFields(t *testing.T) {
	oldVersion, oldCommit, oldBuildTime := Version, Commit, BuildTime
	defer func() {
		Version, Commit, BuildTime = oldVersion, oldCommit, oldBuildTime
	}()

	Version = "v1.2.3"
	Commit = "abc123"
	BuildTime = "2026-05-04T10:00:00Z"

	text := Text()
	for _, want := range []string{"version=v1.2.3", "commit=abc123", "build_time=2026-05-04T10:00:00Z"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected text to contain %q, got %q", want, text)
		}
	}

	fields := Fields()
	if fields["version"] != Version || fields["commit"] != Commit || fields["build_time"] != BuildTime {
		t.Fatalf("unexpected fields: %+v", fields)
	}
}
