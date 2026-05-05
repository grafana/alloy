package opamp

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

const persistentStateFileName = "persistent_state.yaml"

type persistentState struct {
	InstanceID             uuid.UUID           `yaml:"instance_id"`
	LastRemoteConfigStatus *remoteConfigStatus `yaml:"last_remote_config_status,omitempty"`

	configPath string      `yaml:"-"`
	logger     *zap.Logger `yaml:"-"`
}

type remoteConfigStatus struct {
	Status               protobufs.RemoteConfigStatuses `yaml:"status"`
	LastRemoteConfigHash string                         `yaml:"last_remote_config_hash"`
	ErrorMessage         string                         `yaml:"error_message"`
}

func (p *persistentState) SetInstanceID(id uuid.UUID) error {
	p.InstanceID = id
	return p.writeState()
}

func (p *persistentState) SetLastRemoteConfigStatus(status *protobufs.RemoteConfigStatus) error {
	p.LastRemoteConfigStatus = &remoteConfigStatus{
		Status:               status.Status,
		LastRemoteConfigHash: hex.EncodeToString(status.LastRemoteConfigHash),
		ErrorMessage:         status.ErrorMessage,
	}
	return p.writeState()
}

func (p *persistentState) GetLastRemoteConfigStatus() *protobufs.RemoteConfigStatus {
	if p.LastRemoteConfigStatus == nil {
		return nil
	}
	h, err := hex.DecodeString(p.LastRemoteConfigStatus.LastRemoteConfigHash)
	if err != nil {
		p.logger.Error("failed to decode last remote config hash", zap.Error(err))
		return nil
	}
	return &protobufs.RemoteConfigStatus{
		Status:               p.LastRemoteConfigStatus.Status,
		LastRemoteConfigHash: h,
		ErrorMessage:         p.LastRemoteConfigStatus.ErrorMessage,
	}
}

func (p *persistentState) writeState() error {
	b, err := yaml.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(p.configPath, b, 0o600)
}

func loadOrCreatePersistentState(dir string, logger *zap.Logger) (*persistentState, error) {
	path := filepath.Join(dir, persistentStateFileName)
	state, err := loadPersistentState(path, logger)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return createNewPersistentState(path, logger)
	case err != nil:
		return nil, err
	default:
		return state, nil
	}
}

func loadPersistentState(file string, logger *zap.Logger) (*persistentState, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var state persistentState
	if err := yaml.Unmarshal(b, &state); err != nil {
		return nil, err
	}
	state.configPath = file
	state.logger = logger
	return &state, nil
}

func createNewPersistentState(file string, logger *zap.Logger) (*persistentState, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	p := &persistentState{
		InstanceID: id,
		configPath: file,
		logger:     logger,
	}
	if err := p.writeState(); err != nil {
		return nil, err
	}
	return p, nil
}
