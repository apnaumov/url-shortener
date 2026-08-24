package repository

import (
	"maps"
	"sync"
)

type Container[T any] struct {
	mu        sync.Mutex
	container map[string]T
}

func NewContainer[T any]() *Container[T] {
	return &Container[T]{
		container: make(map[string]T),
	}
}

func (store *Container[T]) Get(key string) (T, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	elem, ok := store.container[key]

	if !ok {
		return elem, NotFoundError
	}
	return elem, nil
}

func (store *Container[T]) Set(key string, value T) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	_, ok := store.container[key]
	if !ok {
		store.container[key] = value
		return nil
	}

	return CollisionError
}

func (store *Container[T]) GetAll() map[string]T {
	store.mu.Lock()
	defer store.mu.Unlock()

	return maps.Clone(store.container)
}
