package glock

type Option[T any] func(t *T)

func Apply[T any](t *T, opts ...Option[T]) {
	for _, opt := range opts {
		opt(t)
	}
}

type Mode string

func WithTableName(tableName string) Option[Lock] {
	return func(t *Lock) {
		t.tableName = tableName
	}
}

func WithMode(mode string) Option[Lock] {
	return func(t *Lock) {
		t.mode = mode
	}
}
