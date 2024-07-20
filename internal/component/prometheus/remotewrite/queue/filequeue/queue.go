package filequeue

import "context"

type Queue interface {
	Add(data []byte) (string, error)
	Next(ctx context.Context, enc []byte) ([]byte, string, error)
	Delete(name string)
}
