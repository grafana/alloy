package types

type Data struct {
	Meta map[string]string
	Data []byte
}

type DataHandle struct {
	Name string
	// Pop will get the data and delete the source of the data.
	Pop func() (map[string]string, []byte, error)
}
