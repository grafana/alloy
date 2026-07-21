package util

import "time"

// Step runs fn while logging start, finish (with duration), and any error
// via Logf so the framework's standard prefix is applied. Use it to wrap
// discrete install/cleanup operations whose timing is useful in test logs.
func Step(name string, fn func() error) error {
	start := time.Now()
	Logf("%s...", name)
	if err := fn(); err != nil {
		Logf("failed %s time=%s err=%v", name, time.Since(start).Round(time.Millisecond), err)
		return err
	}
	Logf("done %s time=%s", name, time.Since(start).Round(time.Millisecond))
	return nil
}
