package vault

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/oklog/run"

	vault "github.com/hashicorp/vault/api"
)

func init() {
	component.Register(component.Registration{
		Name:      "remote.vault",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures remote.vault.
type Arguments struct {
	Server    string `alloy:"server,attr"`
	Namespace string `alloy:"namespace,attr,optional"`

	Path string `alloy:"path,attr"`
	Key  string `alloy:"key,attr,optional"`

	RereadFrequency time.Duration `alloy:"reread_frequency,attr,optional"`

	ClientOptions ClientOptions `alloy:"client_options,block,optional"`

	// The user *must* provide exactly one Auth blocks. This must be a slice
	// because the enum flag requires a slice and being tagged as optional.
	//
	// TODO(rfratto): allow the enum flag to be used with a non-slice type.

	Auth []AuthArguments `alloy:"auth,enum,optional"`
}

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	ClientOptions: ClientOptions{
		MinRetryWait: 1000 * time.Millisecond,
		MaxRetryWait: 1500 * time.Millisecond,
		MaxRetries:   2,
		Timeout:      60 * time.Second,
	},
}

// client creates a Vault client from the arguments.
func (a *Arguments) client() (*vault.Client, error) {
	cfg := vault.DefaultConfig()
	cfg.Address = a.Server
	cfg.MinRetryWait = a.ClientOptions.MinRetryWait
	cfg.MaxRetryWait = a.ClientOptions.MaxRetryWait
	cfg.MaxRetries = a.ClientOptions.MaxRetries
	cfg.Timeout = a.ClientOptions.Timeout

	cli, err := vault.NewClient(cfg)
	if err != nil {
		return cli, err
	}

	if a.Namespace != "" {
		cli.SetNamespace(a.Namespace)
	}
	return cli, nil
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Validate implements syntax.Validator.
func (a *Arguments) Validate() error {
	if len(a.Auth) == 0 {
		return fmt.Errorf("exactly one auth.* block must be specified; found none")
	} else if len(a.Auth) > 1 {
		return fmt.Errorf("exactly one auth.* block must be specified; found %d", len(a.Auth))
	}

	if a.ClientOptions.Timeout == 0 {
		return fmt.Errorf("client_options.timeout must be greater than 0")
	}

	return nil
}

func (a *Arguments) authMethod() authMethod {
	if len(a.Auth) != 1 {
		panic(fmt.Sprintf("remote.vault: found %d auth types, expected 1", len(a.Auth)))
	}
	return a.Auth[0].authMethod()
}

func (a *Arguments) secretStore(cli *vault.Client) secretStore {
	// TODO(rfratto): support different stores (like a logical store).
	return &kvStore{c: cli}
}

// ClientOptions sets extra options on the Client.
type ClientOptions struct {
	MinRetryWait time.Duration `alloy:"min_retry_wait,attr,optional"`
	MaxRetryWait time.Duration `alloy:"max_retry_wait,attr,optional"`
	MaxRetries   int           `alloy:"max_retries,attr,optional"`
	Timeout      time.Duration `alloy:"timeout,attr,optional"`
}

// Exports is the values exported by remote.vault.
type Exports struct {
	// Data holds key-value pairs returned from Vault after retrieving the key.
	// Any keys-value pairs returned from Vault which are not []byte or strings
	// cannot be represented as secrets and are therefore ignored.
	//
	// However, it seems that most secrets engines don't actually return
	// arbitrary data, so this limitation shouldn't cause any issues in practice.
	Data map[string]alloytypes.Secret `alloy:"data,attr"`
}

// Component implements the remote.vault component.
type Component struct {
	opts    component.Options
	log     log.Logger
	metrics *metrics

	mut  sync.RWMutex
	args Arguments // Arguments to the component.

	secretManager *tokenManager
	authManager   *tokenManager
}

var (
	_ component.Component       = (*Component)(nil)
	_ component.HealthComponent = (*Component)(nil)
	_ component.DebugComponent  = (*Component)(nil)
)

// New creates a new remote.vault component. It will try to immediately read
// the secret from Vault and return an error if the secret can't be read or if
// authentication against the Vault server fails.
func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:    opts,
		log:     opts.Logger,
		metrics: newMetrics(opts.Registerer),
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run runs the remote.vault component, managing the lifetime of the retrieved
// secret and renewing/rereading it as necessary.
func (c *Component) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var rg run.Group

	rg.Add(func() error {
		c.secretManager.Run(ctx)
		return nil
	}, func(_ error) {
		cancel()
	})

	rg.Add(func() error {
		c.authManager.Run(ctx)
		return nil
	}, func(_ error) {
		cancel()
	})

	return rg.Run()
}

// Update updates the remote.vault component. It will try to immediately read
// the secret from Vault and return an error if the secret can't be read.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	newClient, err := newArgs.client()
	if err != nil {
		return err
	}

	c.mut.Lock()
	c.args = newArgs
	c.mut.Unlock()

	// Configure the token manager for authentication tokens and secrets.
	// authManager *must* be configured first to ensure that the client is
	// authenticated to Vault when retrieving the secret.

	if c.authManager == nil {
		// NOTE(rfratto): we pass 0 for the refresh interval because we don't
		// support refreshing the auth token on an interval.
		mgr, err := newTokenManager(tokenManagerOptions{
			Log:    log.With(c.log, "token_type", "auth"),
			Client: newClient,
			Getter: c.getAuthToken,

			ReadCounter:    c.metrics.authTotal,
			RefreshCounter: c.metrics.authLeaseRenewalTotal,
		})
		if err != nil {
			return err
		}
		c.authManager = mgr
	} else {
		c.authManager.SetClient(newClient)
	}

	if c.secretManager == nil {
		mgr, err := newTokenManager(tokenManagerOptions{
			Log:             log.With(c.log, "token_type", "secret"),
			Client:          newClient,
			Getter:          c.getSecret,
			RefreshInterval: newArgs.RereadFrequency,

			ReadCounter:    c.metrics.secretReadTotal,
			RefreshCounter: c.metrics.secretLeaseRenewalTotal,
		})
		if err != nil {
			return err
		}
		c.secretManager = mgr
	} else {
		c.secretManager.SetClient(newClient)
		c.secretManager.SetRefreshInterval(newArgs.RereadFrequency)
	}

	return nil
}

func (c *Component) getAuthToken(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	c.mut.RLock()
	defer c.mut.RUnlock()

	authMethod := c.args.authMethod()
	return authMethod.vaultAuthenticate(ctx, cli)
}

func (c *Component) getSecret(ctx context.Context, cli *vault.Client) (*vault.Secret, error) {
	c.mut.RLock()
	defer c.mut.RUnlock()

	store := c.args.secretStore(cli)
	secret, err := store.Read(ctx, &c.args)
	if err != nil {
		return nil, err
	}

	// Export the secret so other components can use it.
	c.exportSecret(secret)

	return secret, nil
}

// exportSecret converts the secret into exports and exports it to the
// controller.
func (c *Component) exportSecret(secret *vault.Secret) {
	newExports := Exports{
		Data: make(map[string]alloytypes.Secret),
	}

	for key, value := range secret.Data {
		switch value := value.(type) {
		case string:
			newExports.Data[key] = alloytypes.Secret(value)
		case []byte:
			newExports.Data[key] = alloytypes.Secret(value)

		default:
			// Non-string secrets are ignored.
			level.Warn(c.log).Log(
				"msg", "found field in secret which cannot be converted into a string",
				"key", key,
				"type", fmt.Sprintf("%T", value),
			)
		}
	}

	c.opts.OnStateChange(newExports)
}

// CurrentHealth returns the current health of the remote.vault component. It
// will be healthy as long as the latest read or renewal was successful.
func (c *Component) CurrentHealth() component.Health {
	return component.LeastHealthy(
		c.authManager.CurrentHealth(),
		c.secretManager.CurrentHealth(),
	)
}

// DebugInfo returns debug information about the remote.vault component. It
// includes non-sensitive metadata about the current secret.
func (c *Component) DebugInfo() any {
	return debugInfo{
		AuthToken: c.authManager.DebugInfo(),
		Secret:    c.secretManager.DebugInfo(),
	}
}

type debugInfo struct {
	AuthToken secretInfo `alloy:"auth_token,block"`
	Secret    secretInfo `alloy:"secret,block"`
}
