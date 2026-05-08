package convert

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type testStringer struct{}

func (testStringer) String() string {
	return "stringer"
}

type testPointerStringer struct{}

func (*testPointerStringer) String() string {
	return "pointer stringer"
}

type testStringAlias string

type testIntAlias int64

type testBytesAlias []byte

func TestString(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 10, 11, 12, 13, time.UTC)
	var nilStringer *testPointerStringer
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "nil", value: nil, want: ""},
		{name: "string", value: "hello", want: "hello"},
		{name: "bytes", value: []byte("hello"), want: "hello"},
		{name: "json number", value: json.Number("12.5"), want: "12.5"},
		{name: "time", value: now, want: "2026-05-05T10:11:12.000000013Z"},
		{name: "stringer", value: testStringer{}, want: "stringer"},
		{name: "nil stringer", value: nilStringer, want: ""},
		{name: "error", value: errors.New("boom"), want: "boom"},
		{name: "bool", value: true, want: "true"},
		{name: "int", value: int64(-42), want: "-42"},
		{name: "uint", value: uint64(42), want: "42"},
		{name: "float", value: 3.5, want: "3.5"},
		{name: "string alias", value: testStringAlias("alias"), want: "alias"},
		{name: "int alias", value: testIntAlias(-12), want: "-12"},
		{name: "bytes alias", value: testBytesAlias("alias-bytes"), want: "alias-bytes"},
		{name: "object", value: map[string]any{"name": "kd"}, want: `{"name":"kd"}`},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := String(tt.value); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStringOr(t *testing.T) {
	t.Parallel()

	if got := StringOr(nil, "fallback"); got != "fallback" {
		t.Fatalf("StringOr(nil) = %q, want fallback", got)
	}
	if got := StringOr("value", "fallback"); got != "value" {
		t.Fatalf("StringOr(value) = %q, want value", got)
	}
}

func TestInt(t *testing.T) {
	t.Parallel()

	got, err := Int(" 42 ")
	if err != nil {
		t.Fatalf("Int() error = %v", err)
	}
	if got != 42 {
		t.Fatalf("Int() = %d, want 42", got)
	}
}

func TestIntOr(t *testing.T) {
	t.Parallel()

	if got := IntOr("bad", 7); got != 7 {
		t.Fatalf("IntOr() = %d, want fallback", got)
	}
}

func TestInt64Or(t *testing.T) {
	t.Parallel()

	if got := Int64Or("1700000000", 0); got != 1700000000 {
		t.Fatalf("Int64Or() = %d, want 1700000000", got)
	}
}

func TestUint64(t *testing.T) {
	t.Parallel()

	got, err := Uint64(" 42 ")
	if err != nil {
		t.Fatalf("Uint64() error = %v", err)
	}
	if got != 42 {
		t.Fatalf("Uint64() = %d, want 42", got)
	}
}

func TestFloat64Or(t *testing.T) {
	t.Parallel()

	if got := Float64Or(" 3.5 ", 0); got != 3.5 {
		t.Fatalf("Float64Or() = %f, want 3.5", got)
	}
	if got := Float64Or("bad", 1.5); got != 1.5 {
		t.Fatalf("Float64Or() = %f, want fallback 1.5", got)
	}
}

func TestBoolOr(t *testing.T) {
	t.Parallel()

	if got := BoolOr("true", false); !got {
		t.Fatalf("BoolOr() = false, want true")
	}
	if got := BoolOr("unknown", true); !got {
		t.Fatalf("BoolOr() = false, want fallback true")
	}
}
