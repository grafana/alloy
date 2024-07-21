package filequeue

import "context"

type Storage interface {
	Add(data []byte) (string, error)
	Next(ctx context.Context, enc []byte) ([]byte, string, error)
}
