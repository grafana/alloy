package opampmanager

import (
	"fmt"
	"log"
	"os"
)

// BootReconcile restores effective config from .prev when the last apply did not
// complete cleanly (in theory)
func BootReconcile(effectivePath, statePath string, lg *log.Logger) error {
	st, err := readState(statePath)
	if err != nil {
		return fmt.Errorf("opampmanager: read state: %w", err)
	}
	prevPath := effectivePath + ".prev"
	switch st.Phase {
	case PhasePendingApply, PhaseApplyFailed:
		prev, err := os.ReadFile(prevPath)
		if err != nil {
			if lg != nil {
				lg.Printf("opampmanager: boot reconcile skip restore, missing .prev path=%s err=%v", prevPath, err)
			}
		} else {
			if err := atomicWriteFile(effectivePath, prev, 0o600); err != nil {
				return fmt.Errorf("opampmanager: boot restore effective from .prev: %w", err)
			}
			if lg != nil {
				lg.Printf("opampmanager: boot reconcile restored last-known-good from .prev phase=%s", st.Phase)
			}
		}
		st = State{Phase: PhaseIdle}
		if err := writeStateAtomic(statePath, st); err != nil {
			return fmt.Errorf("opampmanager: write state after boot reconcile: %w", err)
		}
	default:
		// idle / applied_ok — nothing to repair
	}
	return nil
}
