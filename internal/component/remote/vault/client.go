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
	var kvSecret *vault.KVSecret
	var err error

	// If a key is provided, we use that to get the secret from the KV store.
	// This allows for more flexibility in how secrets are stored in vault.
	// eg. long/path/kv/secret/key where long/path/kv is the path and secret/key is the key.
	if args.Key != "" {
		kv := ks.c.KVv2(args.Path)
		kvSecret, err = kv.Get(ctx, args.Key)
		if err != nil {
			return nil, err
		}
	} else {
		// for backward compatibility, we assume the path is in the format path/secret
		// Split the path so we know which kv mount we want to use.
		pathParts := strings.SplitN(args.Path, "/", 2)
		if len(pathParts) != 2 {
			return nil, fmt.Errorf("missing mount path in %q", args.Path)
		}

		kv := ks.c.KVv2(pathParts[0])
		kvSecret, err = kv.Get(ctx, pathParts[1])
		if err != nil {
			return nil, err
		}
	}

	// kvSecret.Data contains unwrapped data. Let's assign that to the raw secret
	// and return it. This is a bit of a hack, but should work just fine.
	kvSecret.Raw.Data = kvSecret.Data
	return kvSecret.Raw, nil
}
