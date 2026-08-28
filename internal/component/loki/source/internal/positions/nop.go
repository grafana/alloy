package positions

import "time"

var _ Positions = (*Nop)(nil)

func NewNop() *Nop {
	return &Nop{}
}

type Nop struct{}

func (n *Nop) Get(path string, labels string) (int64, error) { return 0, nil }

func (n *Nop) GetString(path string, labels string) string { return "" }

func (n *Nop) Put(path string, labels string, pos int64) {}

func (n *Nop) PutString(path string, labels string, pos string) {}

func (n *Nop) Remove(path string, labels string) {}

func (n *Nop) Stop() {}

func (n *Nop) SyncPeriod() time.Duration { return 10 * time.Second }
