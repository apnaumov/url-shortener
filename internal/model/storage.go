package model

import "sync"

type Storage[T any] struct {
	mu        sync.Mutex
	container map[string]T
}

func NewStorage[T any]() *Storage[T] {
	return &Storage[T]{
		container: make(map[string]T),
	}
}

func (store *Storage[T]) Get(key string) (T, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()

	elem, ok := store.container[key]
	return elem, ok
}

func (store *Storage[T]) Set(key string, value T) {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.container[key] = value
}

func (store *Storage[T]) WorkWithContainer(worker func(container map[string]T) error) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	return worker(store.container)
}
