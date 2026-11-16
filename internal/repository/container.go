// Package repository содержит хранилища URL (in-memory и PostgreSQL) и обработчик отложенных сообщений.
package repository

import (
	"maps"
	"sync"
)

// Container — потокобезопасная map со строковыми ключами.
type Container[T any] struct {
	mu        sync.Mutex
	container map[string]T
}

// NewContainer создаёт пустой Container.
func NewContainer[T any]() *Container[T] {
	return &Container[T]{
		container: make(map[string]T),
	}
}

// Get возвращает значение по ключу и признак его наличия.
func (store *Container[T]) Get(key string) (T, bool) {
	store.mu.Lock()
	defer store.mu.Unlock()

	elem, ok := store.container[key]

	return elem, ok
}

// Set добавляет значение, если ключа ещё нет. Возвращает false, если ключ занят.
func (store *Container[T]) Set(key string, value T) bool {
	store.mu.Lock()
	defer store.mu.Unlock()

	_, ok := store.container[key]
	if !ok {
		store.container[key] = value
		return true
	}

	return false
}

// ForceSet записывает значение, перезаписывая существующее.
func (store *Container[T]) ForceSet(key string, value T) {
	store.mu.Lock()
	defer store.mu.Unlock()

	store.container[key] = value
}

// Range вызывает fn для каждой пары под блокировкой контейнера,
// поэтому fn не должна обращаться к этому же Container.
func (store *Container[T]) Range(fn func(key string, value T)) {
	store.mu.Lock()
	defer store.mu.Unlock()

	for k, v := range store.container {
		fn(k, v)
	}
}

// GetAll возвращает копию всего содержимого.
func (store *Container[T]) GetAll() map[string]T {
	store.mu.Lock()
	defer store.mu.Unlock()

	return maps.Clone(store.container)
}
