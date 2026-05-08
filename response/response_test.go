package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONHelpers(t *testing.T) {
	recorder := httptest.NewRecorder()
	OK(recorder, map[string]string{"status": "ok"})

	result := recorder.Result()
	defer result.Body.Close()
	if result.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", result.StatusCode)
	}
	if got := result.Header.Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("unexpected content type: %q", got)
	}

	var envelope Envelope
	if err := json.NewDecoder(result.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Code != CodeOK {
		t.Fatalf("unexpected code: %d", envelope.Code)
	}
}

func TestNewPaginationAppliesDefaults(t *testing.T) {
	pagination := NewPagination(41, 0, 0, []string{"a"})
	if pagination.Page != 1 {
		t.Fatalf("unexpected default page: %d", pagination.Page)
	}
	if pagination.PageSize != 20 {
		t.Fatalf("unexpected default page size: %d", pagination.PageSize)
	}
	if pagination.TotalPages != 3 {
		t.Fatalf("unexpected total pages: %d", pagination.TotalPages)
	}
}
