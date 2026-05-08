package redis

import "testing"

func TestKey(t *testing.T) {
	tests := []struct {
		prefix string
		key    string
		want   string
	}{
		{prefix: "", key: "user:1", want: "user:1"},
		{prefix: "app", key: "user:1", want: "app:user:1"},
		{prefix: "app:", key: "user:1", want: "app:user:1"},
	}

	for _, tt := range tests {
		if got := Key(tt.prefix, tt.key); got != tt.want {
			t.Fatalf("Key(%q, %q) = %q, want %q", tt.prefix, tt.key, got, tt.want)
		}
	}
}
