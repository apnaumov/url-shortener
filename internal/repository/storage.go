package repository

import (
	"errors"
	"sync"
)

type Storage[T any] struct {
	mu        sync.Mutex
	container map[string]T
}

var (
	NotFoundError  = errors.New("doesn't have in storage")
	CollisionError = errors.New("storage already have this key")
)

func NewStorage[T any]() *Storage[T] {
	return &Storage[T]{
		container: make(map[string]T),
	}
}

func (store *Storage[T]) Get(key string) (T, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	elem, ok := store.container[key]

	if !ok {
		return elem, NotFoundError
	}
	return elem, nil
}

func (store *Storage[T]) Set(key string, value T) error {
	store.mu.Lock()
	defer store.mu.Unlock()

	_, ok := store.container[key]
	if !ok {
		store.container[key] = value
		return nil
	}

	return CollisionError
}
