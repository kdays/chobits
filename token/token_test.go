package token

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kdays/chobits/cache"
)

func TestServiceIssueFindAndRevoke(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	service := New(cache.NewMemory(), WithPrefix("test_"), WithTTL(time.Hour))
	service.now = func() time.Time { return now }

	issued, err := service.Issue(ctx, Claims{
		Subject: "user-1",
		Kind:    "session",
		Scopes:  []string{"read", "write"},
		Meta:    map[string]string{"tenant": "kd"},
	})
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	if !strings.HasPrefix(issued.Value, "test_") {
		t.Fatalf("expected custom prefix, got %q", issued.Value)
	}

	claims, err := service.Find(ctx, issued.Value)
	if err != nil {
		t.Fatalf("find token: %v", err)
	}
	if claims.Subject != "user-1" || claims.Kind != "session" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if len(claims.Scopes) != 2 || claims.Scopes[0] != "read" || claims.Scopes[1] != "write" {
		t.Fatalf("unexpected scopes: %+v", claims.Scopes)
	}
	if claims.Meta["tenant"] != "kd" {
		t.Fatalf("unexpected meta: %+v", claims.Meta)
	}

	if err := service.Revoke(ctx, issued.Value); err != nil {
		t.Fatalf("revoke token: %v", err)
	}
	_, err = service.Find(ctx, issued.Value)
	if !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("expected revoked token to miss, got %v", err)
	}
}

func TestServiceRejectsInvalidAndExpiredTokens(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	service := New(newTestStore(), WithTTL(time.Hour))
	service.now = func() time.Time { return now }

	if _, err := service.IssueSubject(ctx, " "); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid subject error, got %v", err)
	}

	if _, err := service.Issue(ctx, Claims{
		Subject: "user-1",
		Expires: now.Add(-time.Second).Unix(),
	}); !errors.Is(err, ErrExpired) {
		t.Fatalf("expected expired issue error, got %v", err)
	}

	issued, err := service.Issue(ctx, Claims{
		Subject: "user-1",
		Expires: now.Add(time.Second).Unix(),
	})
	if err != nil {
		t.Fatalf("issue expiring token: %v", err)
	}
	now = now.Add(2 * time.Second)

	_, err = service.Find(ctx, issued.Value)
	if !errors.Is(err, ErrExpired) {
		t.Fatalf("expected expired find error, got %v", err)
	}
	if _, err := service.store.Get(ctx, service.key(issued.Value)); !errors.Is(err, cache.ErrMiss) {
		t.Fatalf("expected expired token to be revoked, got %v", err)
	}
}

type testStore struct {
	values map[string][]byte
}

func newTestStore() *testStore {
	return &testStore{values: make(map[string][]byte)}
}

func (store *testStore) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := store.values[key]
	if !ok {
		return nil, cache.ErrMiss
	}
	copied := append([]byte(nil), value...)
	return copied, nil
}

func (store *testStore) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	store.values[key] = append([]byte(nil), value...)
	return nil
}

func (store *testStore) Take(_ context.Context, key string) ([]byte, error) {
	value, err := store.Get(context.Background(), key)
	if err != nil {
		return nil, err
	}
	delete(store.values, key)
	return value, nil
}

func (store *testStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := store.values[key]
	return ok, nil
}

func (store *testStore) Increment(_ context.Context, _ string, _ int64, _ time.Duration) (int64, error) {
	return 0, errors.New("increment unsupported in test store")
}

func (store *testStore) TTL(_ context.Context, key string) (time.Duration, error) {
	if _, ok := store.values[key]; !ok {
		return 0, cache.ErrMiss
	}
	return 0, nil
}

func (store *testStore) Delete(_ context.Context, key string) error {
	delete(store.values, key)
	return nil
}

func (store *testStore) Close() error {
	return nil
}
