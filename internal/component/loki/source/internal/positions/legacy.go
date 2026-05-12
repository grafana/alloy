package positions

import (
	"os"
	"path/filepath"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"gopkg.in/yaml.v2"
)

// LegacyFile is the file format used by agent static mode.
type LegacyFile struct {
	Positions map[string]string `yaml:"positions"`
}

// ConvertLegacyPositionsFile will convert the legacy positions file to the new format if:
// 1. There is no file at the newpath
// 2. There is a file at the legacy path and that it is valid yaml
func ConvertLegacyPositionsFile(legacyPath, newPath string, l log.Logger) {
	legacyPositions := readLegacyFile(legacyPath, l)
	// legacyPositions did not exist or was invalid so return.
	if legacyPositions == nil {
		level.Info(l).Log("msg", "will not convert the legacy positions file as it is not valid or does not exist", "legacy_path", legacyPath)
		return
	}
	fi, err := os.Stat(newPath)
	// If the newpath exists, then don't convert.
	if err == nil && fi.Size() > 0 {
		level.Info(l).Log("msg", "will not convert the legacy positions file as the new positions file already exists", "path", newPath)
		return
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
		level.Error(l).Log("msg", "error writing new positions file converted from legacy", "path", newPath, "error", err)
	}
	level.Info(l).Log("msg", "successfully converted legacy positions file to the new format", "path", newPath, "legacy_path", legacyPath)
}

// ConvertLegacyPositionsFileJournal will convert the legacy positions file to the new format for a journal job if:
// 1. There is no file at the newpath
// 2. There is a file at the legacy path and that it is valid yaml
//
// legacyJob is the name of the journal job in e.g. promtail or agent static.
func ConvertLegacyPositionsFileJournal(legacyPath, legacyJob string, newPath string, componentID string, l log.Logger) {
	legacyPositions := readLegacyFile(legacyPath, l)
	// legacyPositions did not exist or was invalid so return.
	if legacyPositions == nil {
		level.Info(l).Log("msg", "will not convert the legacy positions file as it is not valid or does not exist", "legacy_path", legacyPath)
		return
	}
	fi, err := os.Stat(newPath)
	// If the newpath exists, then don't convert.
	if err == nil && fi.Size() > 0 {
		level.Info(l).Log("msg", "will not convert the legacy positions file as the new positions file already exists", "path", newPath)
		return
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
		level.Error(l).Log("msg", "error writing new positions file converted from legacy", "path", newPath, "error", err)
	}
	level.Info(l).Log("msg", "successfully converted legacy positions file to the new format", "path", newPath, "legacy_path", legacyPath)
}

func readLegacyFile(legacyPath string, l log.Logger) *LegacyFile {
	oldFile, err := os.Stat(legacyPath)
	// If the old file doesn't exist or is empty then return early.
	if err != nil || oldFile.Size() == 0 {
		level.Info(l).Log("msg", "no legacy positions file found", "path", legacyPath)
		return nil
	}
	// Try to read and parse the legacy file.
	clean := filepath.Clean(legacyPath)
	buf, err := os.ReadFile(clean)
	if err != nil {
		level.Error(l).Log("msg", "error reading legacy positions file", "path", clean, "error", err)
		return nil
	}
	legacyPositions := &LegacyFile{}
	err = yaml.UnmarshalStrict(buf, legacyPositions)
	if err != nil {
		level.Error(l).Log("msg", "error parsing legacy positions file", "path", clean, "error", err)
		return nil
	}
	return legacyPositions
}
