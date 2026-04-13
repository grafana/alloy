package opampmanager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ApplyPhase tracks crash-safe apply lifecycle on disk.
type ApplyPhase string

const (
	PhaseIdle         ApplyPhase = "idle"
	PhasePendingApply ApplyPhase = "pending_apply"
	PhaseAppliedOK    ApplyPhase = "applied_ok"
	PhaseApplyFailed  ApplyPhase = "apply_failed"
)

// State is persisted at the managed config state_path (JSON).
type State struct {
	Phase         ApplyPhase `json:"phase"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CandidateHash string     `json:"candidate_hash,omitempty"`
}

func readState(path string) (State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{Phase: PhaseIdle}, nil
		}
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return State{}, err
	}
	if s.Phase == "" {
		s.Phase = PhaseIdle
	}
	return s, nil
}

func writeStateAtomic(path string, s State) error {
	s.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(&s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".opampstate-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
