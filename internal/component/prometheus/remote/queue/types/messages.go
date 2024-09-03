package types

type Data struct {
	Meta map[string]string
	Data []byte
}

type DataHandle struct {
	Name string
	Get  func() (map[string]string, []byte, error)
}
