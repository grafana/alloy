package positions

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// LegacyFile is the copied struct for the static mode positions file.
type LegacyFile struct {
	Positions map[string]string `yaml:"positions"`
}

// ConvertLegacyPositionsFile will convert the legacy positions file to the new format if:
// 1. There is no file at the newpath
// 2. There is a file at the legacy path and that it is valid yaml
//
// Returns an error if the legacy file exists but couldn't be read (#5493).
func ConvertLegacyPositionsFile(legacyPath, newPath string, l *slog.Logger) error {
	legacyPositions, err := readLegacyFile(legacyPath, l)
	if err != nil {
		return fmt.Errorf("legacy positions file %q exists but could not be read, refusing to start to avoid re-ingesting logs: %w", legacyPath, err)
	}
	// legacyPositions did not exist or was invalid so return.
	if legacyPositions == nil {
		l.Info("will not convert the legacy positions file as it is not valid or does not exist", "legacy_path", legacyPath)
		return nil
	}
	fi, err := os.Stat(newPath)
	// If the newpath exists, then don't convert.
	if err == nil && fi.Size() > 0 {
		l.Info("will not convert the legacy positions file as the new positions file already exists", "path", newPath)
		return nil
	}

	newPositions := make(map[Entry]string)
	for k, v := range legacyPositions.Positions {
		newPositions[Entry{
			Path: k,
			// This is a map of labels but must be an empty map since that is what the new positions expects.
			Labels: "{}",
		}] = v
	}
	err = writePositionFile(newPath, newPositions)
	if err != nil {
		l.Error("error writing new positions file converted from legacy", "path", newPath, "error", err)
	}
	l.Info("successfully converted legacy positions file to the new format", "path", newPath, "legacy_path", legacyPath)
	return nil
}

// ConvertLegacyPositionsFileJournal will convert the legacy positions file to the new format for a journal job if:
// 1. There is no file at the newpath
// 2. There is a file at the legacy path and that it is valid yaml
//
// legacyJob is the name of the journal job in e.g. promatil or agent static.
func ConvertLegacyPositionsFileJournal(legacyPath, legacyJob string, newPath string, componentID string, l *slog.Logger) error {
	legacyPositions, err := readLegacyFile(legacyPath, l)
	if err != nil {
		return fmt.Errorf("legacy positions file %q exists but could not be read, refusing to start to avoid re-ingesting logs: %w", legacyPath, err)
	}
	// legacyPositions did not exist or was invalid so return.
	if legacyPositions == nil {
		l.Info("will not convert the legacy positions file as it is not valid or does not exist", "legacy_path", legacyPath)
		return nil
	}
	fi, err := os.Stat(newPath)
	// If the newpath exists, then don't convert.
	if err == nil && fi.Size() > 0 {
		l.Info("will not convert the legacy positions file as the new positions file already exists", "path", newPath)
		return nil
	}

	var (
		legacyCursor = CursorKey(legacyJob)
		newCursor    = CursorKey(componentID)
	)

	newPositions := make(map[Entry]string)
	for k, v := range legacyPositions.Positions {
		if k == legacyCursor {
			newPositions[Entry{
				Path:   newCursor,
				Labels: "{}",
			}] = v
			break
		}
	}
	err = writePositionFile(newPath, newPositions)
	if err != nil {
		l.Error("error writing new positions file converted from legacy", "path", newPath, "error", err)
	}
	l.Info("successfully converted legacy positions file to the new format", "path", newPath, "legacy_path", legacyPath)
	return nil
}

// readLegacyFile reads and parses the legacy positions file.
//
// Returns (nil, nil) if the file doesn't exist or is empty. Returns (nil,
// err) if it exists but a genuine I/O error (e.g. permission denied)
// prevented reading it (#5493). Malformed YAML remains non-fatal.
func readLegacyFile(legacyPath string, l *slog.Logger) (*LegacyFile, error) {
	oldFile, err := os.Stat(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			l.Info("no legacy positions file found", "path", legacyPath)
			return nil, nil
		}
		// Real error, not "doesn't exist".
		return nil, fmt.Errorf("error checking legacy positions file %q: %w", legacyPath, err)
	}
	if oldFile.Size() == 0 {
		l.Info("no legacy positions file found", "path", legacyPath)
		return nil, nil
	}
	// Try to read and parse the legacy file.
	clean := filepath.Clean(legacyPath)
	buf, err := os.ReadFile(clean)
	if err != nil {
		// Exists but couldn't be read, e.g. permission denied (#5493).
		return nil, fmt.Errorf("error reading legacy positions file %q: %w", clean, err)
	}
	legacyPositions := &LegacyFile{}
	err = yaml.UnmarshalStrict(buf, legacyPositions)
	if err != nil {
		// Malformed content remains non-fatal; only I/O errors are in scope.
		l.Error("error parsing legacy positions file", "path", clean, "error", err)
		return nil, nil
	}
	return legacyPositions, nil
}
