package scrape

import (
	"fmt"
	"strings"
)

// validateProfileData performs validation to ensure data is a valid pprof profile.
func validateProfileData(data []byte, contentType string) error {
	if len(data) == 0 {
		return fmt.Errorf("empty response from profiling endpoint")
	}

	if err := validateContentType(contentType); err != nil {
		return err
	}

	if isTextContent(data) {
		preview := getFirstBytes(data, 50)
		return fmt.Errorf("text response instead of pprof: %q (server should return HTTP error, not 200)", preview)
	}

	return validateBinaryContent(data)
}

// validateContentType checks HTTP Content-Type header
func validateContentType(contentType string) error {
	if contentType == "" {
		return fmt.Errorf("missing Content-Type header")
	}

	if !strings.HasPrefix(contentType, "application/octet-stream") {
		return fmt.Errorf("unexpected Content-Type %s, expected application/octet-stream for pprof data", contentType)
	}

	return nil
}

// getFirstBytes returns the first n bytes as a string for debugging
func getFirstBytes(data []byte, n int) string {
	if len(data) > n {
		return string(data[:n])
	}
	return string(data)
}

// getHexBytes returns the first n bytes as hex-encoded string for debugging binary data
func getHexBytes(data []byte, n int) string {
	end := min(len(data), n)
	return fmt.Sprintf("%x", data[:end])
}

// isWhitespace checks if a byte is whitespace
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// findFirstNonWhitespace finds the first non-whitespace character.
func findFirstNonWhitespace(data []byte) byte {
	for i := 0; i < len(data) && i < 64; i++ {
		c := data[i]
		if !isWhitespace(c) {
			return c
		}
	}
	return 0
}

// isTextContent uses some heuristics to detect text content without allocations.
func isTextContent(data []byte) bool {
	// Only check longer responses that are likely to be error pages
	if len(data) < 10 {
		return false
	}

	if hasHTMLPrefix(data) {
		return true
	}

	// JSON detection
	first := findFirstNonWhitespace(data)
	if (first == '{' || first == '[') && len(data) > 20 {
		return true
	}

	return false
}

// hasHTMLPrefix checks for HTML prefixes without allocations, skipping leading whitespace.
func hasHTMLPrefix(data []byte) bool {
	if len(data) < 2 {
		return false
	}

	// Skip leading whitespace to handle cases like "\n  <!DOCTYPE html>"
	start := 0
	for start < len(data) && isWhitespace(data[start]) {
		start++
	}

	if start >= len(data) || data[start] != '<' {
		return false
	}

	// Adjust data to start from the '<' character
	data = data[start:]
	if len(data) < 2 {
		return false
	}

	// Check for DOCTYPE
	if len(data) >= 9 && data[1] == '!' && (data[2] == 'D' || data[2] == 'd') {
		return true
	}

	// Check for common HTML tags
	if len(data) >= 5 {
		second := data[1]
		if (second == 'h' || second == 'H') ||
			(second == 'b' || second == 'B') ||
			(second == 't' || second == 'T') { // <title>

			return true
		}
	}

	return false
}

// validateBinaryContent validates binary format with simple checks
func validateBinaryContent(data []byte) error {
	if len(data) <= 2 {
		return fmt.Errorf("data too short for pprof format")
	}

	if isGzipData(data) {
		return nil
	}

	// Check protobuf wireType in first byte. Valid wireTypes are 0-5, invalids are 6-7.
	// This catches corrupted data that would cause "proto: illegal wireType 6/7" errors later.
	if (data[0] & 0x07) >= 6 {
		preview := getHexBytes(data, 16)
		return fmt.Errorf("invalid protobuf data: illegal wireType %d (expected 0-5), first bytes: %s", data[0]&0x07, preview)
	}

	return nil
}
