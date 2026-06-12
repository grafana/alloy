package syncreplaces

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
)

type builderConfigScanner struct {
	scanner    *bufio.Scanner
	lineNumber int
}

func newBuilderConfigScanner(builderConfig []byte) *builderConfigScanner {
	return &builderConfigScanner{
		scanner: bufio.NewScanner(bytes.NewReader(builderConfig)),
	}
}

func (s *builderConfigScanner) findSharedReplacesStart() error {
	for s.scan() {
		trimmed := strings.TrimSpace(s.scanner.Text())
		if isCommentMarker(trimmed, builderConfigSharedReplacesStart) {
			return nil
		}
	}
	if err := s.scanner.Err(); err != nil {
		return fmt.Errorf("scan builder config: %w", err)
	}
	return fmt.Errorf("missing shared replace markers")
}

func (s *builderConfigScanner) readSharedReplaceEntry() (*replaceEntry, error) {
	var comments []string
	for s.scan() {
		trimmed := strings.TrimSpace(s.scanner.Text())
		if trimmed == "" {
			continue
		}
		if isCommentMarker(trimmed, builderConfigSharedReplacesEnd) {
			return nil, nil
		}
		if after, ok := strings.CutPrefix(trimmed, "#"); ok {
			comments = append(comments, strings.TrimSpace(after))
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			return nil, fmt.Errorf("line %d in shared replace block must be a comment or replace entry", s.lineNumber)
		}
		entry, err := newReplaceEntry(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), comments)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", s.lineNumber, err)
		}
		return &entry, nil
	}
	if err := s.scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan builder config: %w", err)
	}
	return nil, fmt.Errorf("missing shared replace end marker")
}

func (s *builderConfigScanner) scan() bool {
	if !s.scanner.Scan() {
		return false
	}
	s.lineNumber++
	return true
}

func isCommentMarker(line string, marker string) bool {
	return strings.HasPrefix(line, "#") && strings.Contains(line, marker)
}
