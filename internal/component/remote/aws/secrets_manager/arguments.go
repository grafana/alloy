package secrets_manager

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	awscommon "github.com/grafana/alloy/internal/component/common/config/aws"
)

// minPollFrequency limits API usage to limit costs and avoid API rate limits.
const minPollFrequency = time.Minute

// Arguments configures remote.aws.secrets_manager.
type Arguments struct {
	SecretID      string           `alloy:"secret_id,attr"`
	VersionStage  string           `alloy:"version_stage,attr,optional"`
	VersionID     string           `alloy:"version_id,attr,optional"`
	PollFrequency time.Duration    `alloy:"poll_frequency,attr,optional"`
	Client        awscommon.Client `alloy:"client,block,optional"`
}

// DefaultArguments holds the default settings for Arguments.
var DefaultArguments = Arguments{
	PollFrequency: time.Hour,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if a.SecretID == "" {
		return errors.New("secret_id must not be empty")
	}
	if a.VersionStage != "" && a.VersionID != "" {
		return errors.New("only one of version_stage and version_id may be set")
	}
	if a.PollFrequency < 0 || (a.PollFrequency > 0 && a.PollFrequency < minPollFrequency) {
		return fmt.Errorf("poll_frequency must be 0 or at least %s", minPollFrequency)
	}
	return a.Client.Validate()
}

func (a Arguments) input() *secretsmanager.GetSecretValueInput {
	in := &secretsmanager.GetSecretValueInput{SecretId: aws.String(a.SecretID)}
	if a.VersionStage != "" {
		in.VersionStage = aws.String(a.VersionStage)
	}
	if a.VersionID != "" {
		in.VersionId = aws.String(a.VersionID)
	}
	return in
}
