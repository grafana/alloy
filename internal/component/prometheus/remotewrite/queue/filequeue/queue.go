package filequeue

type MetricQueue interface {
	Add(data []byte) (string, error)
	Next(enc []byte) ([]byte, string, bool, bool)
	Name() string
}
