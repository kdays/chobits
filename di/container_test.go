package di

import (
	"errors"
	"testing"
)

type sampleComponent struct {
	name string
}

type closeComponent struct {
	closed bool
}

func (component *closeComponent) Close() error {
	component.closed = true
	return nil
}

type secondaryCloseComponent struct{}

func (component *secondaryCloseComponent) Close() error {
	return nil
}

func TestRegisterResolveAndDuplicate(t *testing.T) {
	container := New()
	component := &sampleComponent{name: "primary"}
	if err := Register(container, component); err != nil {
		t.Fatalf("register component: %v", err)
	}

	resolved, err := Resolve[*sampleComponent](container)
	if err != nil {
		t.Fatalf("resolve component: %v", err)
	}
	if resolved != component {
		t.Fatalf("resolved unexpected component: %p want %p", resolved, component)
	}

	if err := Register(container, &sampleComponent{name: "duplicate"}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if _, err := Resolve[*int](container); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestNamedComponents(t *testing.T) {
	container := New()
	component := &sampleComponent{name: "named"}
	MustRegisterName(container, "service", component)

	resolved, err := ResolveName[*sampleComponent](container, "service")
	if err != nil {
		t.Fatalf("resolve named component: %v", err)
	}
	if resolved != component {
		t.Fatalf("resolved unexpected named component: %p want %p", resolved, component)
	}

	if err := RegisterName(container, "service", &sampleComponent{}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected named duplicate error, got %v", err)
	}
	if _, err := ResolveName[*sampleComponent](container, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected named not found error, got %v", err)
	}
}

func TestLifecycleCollectionCanBeDisabled(t *testing.T) {
	container := New()
	closer := &closeComponent{}
	if err := Register(container, closer); err != nil {
		t.Fatalf("register closer: %v", err)
	}
	if got := len(container.Closers()); got != 1 {
		t.Fatalf("expected one collected closer, got %d", got)
	}

	secondary := &secondaryCloseComponent{}
	if err := RegisterNoLifecycle(container, secondary); err != nil {
		t.Fatalf("register no-lifecycle closer: %v", err)
	}
	if got := len(container.Closers()); got != 1 {
		t.Fatalf("expected no-lifecycle registration not to collect closer, got %d", got)
	}

	container.AddCloser(secondary)
	if got := len(container.Closers()); got != 2 {
		t.Fatalf("expected explicit closer to be collected, got %d", got)
	}
}
