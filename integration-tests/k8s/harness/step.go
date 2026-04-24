package harness

import (
	"fmt"
	"time"
)

func step(name string, fn func() error) error {
	start := time.Now()
	fmt.Printf("[k8s-itest] %s...\n", name)
	err := fn()
	if err != nil {
		fmt.Printf("[k8s-itest] failed %s time=%s err=%v\n", name, time.Since(start).Round(time.Millisecond), err)
		return err
	}
	fmt.Printf("[k8s-itest] done %s time=%s\n", name, time.Since(start).Round(time.Millisecond))
	return nil
}
