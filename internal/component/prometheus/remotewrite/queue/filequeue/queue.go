package filequeue

type Queue interface {
	Add(data []byte) (string, error)
	Next(enc []byte) ([]byte, string, bool, bool)
	Name() string
}
