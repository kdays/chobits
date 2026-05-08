package di

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/kdays/chobits/app"
)

var (
	ErrNotFound  = fmt.Errorf("component not found")
	ErrDuplicate = fmt.Errorf("component already registered")
)

type Container struct {
	mu          sync.RWMutex
	values      map[reflect.Type]any
	names       map[string]any
	backgrounds []any
	closers     []app.Closer
}

func New() *Container {
	return &Container{
		values: make(map[reflect.Type]any),
		names:  make(map[string]any),
	}
}

func Register[T any](container *Container, value T) error {
	return register(container, value, true)
}

func RegisterNoLifecycle[T any](container *Container, value T) error {
	return register(container, value, false)
}

func register[T any](container *Container, value T, collectLifecycle bool) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}
	typ := reflect.TypeFor[T]()
	if typ == nil {
		return fmt.Errorf("component type is nil")
	}

	container.mu.Lock()
	defer container.mu.Unlock()

	if _, exists := container.values[typ]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicate, typ.String())
	}
	container.values[typ] = value
	if collectLifecycle {
		container.collectLifecycle(value)
	}
	return nil
}

func Replace[T any](container *Container, value T) {
	if container == nil {
		return
	}
	typ := reflect.TypeFor[T]()
	if typ == nil {
		return
	}

	container.mu.Lock()
	defer container.mu.Unlock()

	container.values[typ] = value
	container.collectLifecycle(value)
}

func MustRegister[T any](container *Container, value T) {
	if err := Register(container, value); err != nil {
		panic(err)
	}
}

func MustRegisterNoLifecycle[T any](container *Container, value T) {
	if err := RegisterNoLifecycle(container, value); err != nil {
		panic(err)
	}
}

func Resolve[T any](container *Container) (T, error) {
	var zero T
	if container == nil {
		return zero, fmt.Errorf("container is nil")
	}
	typ := reflect.TypeFor[T]()
	if typ == nil {
		return zero, fmt.Errorf("component type is nil")
	}

	container.mu.RLock()
	value, ok := container.values[typ]
	container.mu.RUnlock()
	if !ok {
		return zero, fmt.Errorf("%w: %s", ErrNotFound, typ.String())
	}

	typed, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf("component %s has unexpected type %T", typ.String(), value)
	}
	return typed, nil
}

func MustResolve[T any](container *Container) T {
	value, err := Resolve[T](container)
	if err != nil {
		panic(err)
	}
	return value
}

func RegisterName(container *Container, name string, value any) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}
	if name == "" {
		return fmt.Errorf("component name is empty")
	}

	container.mu.Lock()
	defer container.mu.Unlock()

	if _, exists := container.names[name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicate, name)
	}
	container.names[name] = value
	container.collectLifecycle(value)
	return nil
}

func MustRegisterName(container *Container, name string, value any) {
	if err := RegisterName(container, name, value); err != nil {
		panic(err)
	}
}

func ReplaceName(container *Container, name string, value any) {
	if container == nil || name == "" {
		return
	}

	container.mu.Lock()
	defer container.mu.Unlock()

	container.names[name] = value
	container.collectLifecycle(value)
}

func ResolveName[T any](container *Container, name string) (T, error) {
	var zero T
	if container == nil {
		return zero, fmt.Errorf("container is nil")
	}
	if name == "" {
		return zero, fmt.Errorf("component name is empty")
	}

	container.mu.RLock()
	value, ok := container.names[name]
	container.mu.RUnlock()
	if !ok {
		return zero, fmt.Errorf("%w: %s", ErrNotFound, name)
	}

	typed, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf("component %s has unexpected type %T", name, value)
	}
	return typed, nil
}

func MustResolveName[T any](container *Container, name string) T {
	value, err := ResolveName[T](container, name)
	if err != nil {
		panic(err)
	}
	return value
}

func (container *Container) AddBackground(service any) {
	if container == nil || service == nil {
		return
	}
	container.mu.Lock()
	defer container.mu.Unlock()
	container.backgrounds = append(container.backgrounds, service)
}

func (container *Container) AddCloser(closer app.Closer) {
	if container == nil || closer == nil {
		return
	}
	container.mu.Lock()
	defer container.mu.Unlock()
	container.closers = append(container.closers, closer)
}

func (container *Container) Backgrounds() []any {
	if container == nil {
		return nil
	}
	container.mu.RLock()
	defer container.mu.RUnlock()
	return append([]any(nil), container.backgrounds...)
}

func (container *Container) Closers() []app.Closer {
	if container == nil {
		return nil
	}
	container.mu.RLock()
	defer container.mu.RUnlock()
	return append([]app.Closer(nil), container.closers...)
}

func (container *Container) collectLifecycle(value any) {
	if value == nil {
		return
	}
	if _, ok := value.(app.Background); ok {
		container.backgrounds = append(container.backgrounds, value)
	} else if _, ok := value.(app.Stopper); ok {
		container.backgrounds = append(container.backgrounds, value)
	} else if _, ok := value.(app.Waiter); ok {
		container.backgrounds = append(container.backgrounds, value)
	}
	if closer, ok := value.(app.Closer); ok {
		container.closers = append(container.closers, closer)
	}
}
