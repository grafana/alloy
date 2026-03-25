package loki

import (
	"context"
	"reflect"
)

type Consumer interface {
	Consume(ctx context.Context, batch Batch) error
	ConsumeEntry(ctx context.Context, entry Entry) error
}

func requireUpdate[T any](prev, next []T) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}
