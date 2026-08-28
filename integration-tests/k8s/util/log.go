package util

import "fmt"

// LogPrefix marks lines emitted by the framework (runner / harness / deps).
const LogPrefix = "[k8s-itest] "

// Logf prints LogPrefix + formatted message + newline.
func Logf(format string, args ...any) {
	fmt.Printf(LogPrefix+format+"\n", args...)
}
