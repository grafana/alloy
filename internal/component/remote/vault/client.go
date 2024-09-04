package vault

import (
	"context"
	"fmt"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

// secretStore abstracts away the details for how a secret is retrieved from a
// vault.Client.
type secretStore interface {
	Read(ctx context.Context, args *Arguments) (*vault.Secret, error)
}

// TODO(rfratto): support logical stores.

type kvStore struct{ c *vault.Client }

func (ks *kvStore) Read(ctx context.Context, args *Arguments) (*vault.Secret, error) {
	kv := ks.c.KVv2(args.Path)
	kvSecret, err := kv.Get(ctx, args.Secret)
	if err != nil {
		return nil, err
	}

	// kvSecret.Data contains unwrapped data. Let's assign that to the raw secret
	// and return it. This is a bit of a hack, but should work just fine.
	kvSecret.Raw.Data = kvSecret.Data
	return kvSecret.Raw, nil
}
