package main

//go:noinline
func burnCPU(n uint64) uint64 {
	for range 1_000_000 {
		n = n*6364136223846793005 + 1442695040888963407
	}
	return n
}

func main() {
	n := uint64(1)
	for {
		n = burnCPU(n)
	}
}
