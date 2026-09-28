package discovery

import (
	"testing"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/require"
)

func TestDistributedTargets_SeparateIdentityAndOwnership(t *testing.T) {
	first := mkTarget("__address__", "localhost:9090", "__param_sig", "first", "__meta_source", "one")
	duplicate := mkTarget("__address__", "localhost:9090", "__param_sig", "first", "__meta_source", "two")
	second := mkTarget("__address__", "localhost:9090", "__param_sig", "second")
	for _, enabled := range []bool{false, true} {
		dt := NewDistributedTargets(enabled, &fakeCluster{}, []Target{first, duplicate, second, first}, TargetHashing{Ownership: addressHash})
		require.Equal(t, []Target{first, second}, dt.LocalTargets())
		require.Equal(t, 2, dt.TargetCount())
	}

	// Existing allowlist consumers intentionally deduplicate on their selected labels.
	dt := NewDistributedTargets(true, &fakeCluster{}, []Target{first, second}, TargetHashing{Identity: addressHash})
	require.Equal(t, []Target{first}, dt.LocalTargets())
	require.Equal(t, 1, dt.TargetCount())
}

func TestDistributedTargets_CustomOwnershipDisabled(t *testing.T) {
	key := keyFor(EmptyTarget)
	c := &fakeCluster{peers: allTestPeers, lookupMap: map[shard.Key][]peer.Peer{key: {peer2}}}
	dt := NewDistributedTargets(false, c, allTestTargets, TargetHashing{Ownership: func(Target) uint64 { return uint64(key) }})
	require.Equal(t, allTestTargets, dt.LocalTargets())
	dt = NewDistributedTargets(true, nil, allTestTargets, TargetHashing{Ownership: func(Target) uint64 { return uint64(key) }})
	require.Equal(t, allTestTargets, dt.LocalTargets())
}

func TestDistributedTargets_GroupedHandoff(t *testing.T) {
	first := mkTarget("__address__", "localhost:9090", "database", "one")
	second := mkTarget("__address__", "localhost:9090", "database", "two")
	targets := []Target{first, second}
	hashing := TargetHashing{Ownership: addressHash}
	ownershipKey := keyFor(mkTarget("__address__", "localhost:9090"))
	c := &fakeCluster{peers: allTestPeers, lookupMap: map[shard.Key][]peer.Peer{ownershipKey: {peer1Self}}}
	previous := NewDistributedTargets(true, c, targets, hashing)

	c.lookupMap[ownershipKey] = []peer.Peer{peer2}
	current := NewDistributedTargets(true, c, targets, hashing)
	require.Equal(t, targets, current.MovedToRemoteInstance(previous))
	require.Equal(t, 2, current.TargetCount())

	// Suppress staleness for the entire group, including a disappeared member.
	current = NewDistributedTargets(true, c, []Target{second}, hashing)
	require.Equal(t, targets, current.MovedToRemoteInstance(previous))
	require.Equal(t, 1, current.TargetCount())

	// If the entire group disappears, normal staleness handling still applies.
	current = NewDistributedTargets(true, c, nil, hashing)
	require.Empty(t, current.MovedToRemoteInstance(previous))

	// A refreshed credential changes identity but not group ownership.
	refreshed := mkTarget("__address__", "localhost:9090", "database", "refreshed")
	current = NewDistributedTargets(true, c, []Target{refreshed}, hashing)
	require.Equal(t, targets, current.MovedToRemoteInstance(previous))

	// Changing the ownership hash can change ownership without changing identity.
	c.lookupMap[keyFor(first)] = []peer.Peer{peer2}
	c.lookupMap[keyFor(second)] = []peer.Peer{peer1Self}
	current = NewDistributedTargets(true, c, targets, TargetHashing{})
	require.Equal(t, []Target{first}, current.MovedToRemoteInstance(previous))
	require.Equal(t, []Target{second}, current.LocalTargets())
}

func addressHash(target Target) uint64 {
	return target.SpecificLabelsHash([]string{"__address__"})
}

func TestTargetHashingPresets(t *testing.T) {
	target := mkTarget("__address__", "localhost:9090", "__param_sig", "signature", "__meta_source", "source")
	for _, tc := range []struct {
		name                string
		hashing             TargetHashing
		identity, ownership uint64
	}{
		{"default", TargetHashing{}, target.NonMetaLabelsHash(), target.NonMetaLabelsHash()},
		{"nil identity labels", IdentityLabels(nil), target.NonMetaLabelsHash(), target.NonMetaLabelsHash()},
		{"empty identity labels", IdentityLabels([]string{}), target.NonMetaLabelsHash(), target.NonMetaLabelsHash()},
		{"selected metadata", IdentityLabels([]string{"__meta_source"}), target.SpecificLabelsHash([]string{"__meta_source"}), target.SpecificLabelsHash([]string{"__meta_source"})},
		{"missing identity labels", IdentityLabels([]string{"missing"}), EmptyTarget.NonMetaLabelsHash(), EmptyTarget.NonMetaLabelsHash()},
		{"nil exclusions", ExcludeOwnershipLabels(nil), target.NonMetaLabelsHash(), target.NonMetaLabelsHash()},
		{"empty exclusions", ExcludeOwnershipLabels([]string{}), target.NonMetaLabelsHash(), target.NonMetaLabelsHash()},
		{"excluded signature", ExcludeOwnershipLabels([]string{"__param_sig"}), target.NonMetaLabelsHash(), addressHash(target)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var keys []shard.Key
			c := &fakeCluster{onLookup: func(key shard.Key) { keys = append(keys, key) }}
			dt := NewDistributedTargets(true, c, []Target{target}, tc.hashing)
			require.Equal(t, []shard.Key{shard.Key(tc.ownership)}, keys)
			require.Equal(t, []Target{target}, dt.LocalTargets())
			identity := tc.hashing.Identity
			if identity == nil {
				identity = Target.NonMetaLabelsHash
			}
			require.Equal(t, tc.identity, identity(target))
		})
	}

	// A caller changing its label slice must not change a stored hashing policy.
	names := []string{"__param_sig"}
	excluded := ExcludeOwnershipLabels(names)
	selected := IdentityLabels(names)
	names[0] = "__address__"
	require.Equal(t, addressHash(target), excluded.Ownership(target))
	require.Equal(t, target.SpecificLabelsHash([]string{"__param_sig"}), selected.Identity(target))
}

func TestDistributedTargets_CustomOwnershipReadinessAndFailures(t *testing.T) {
	hashing := ExcludeOwnershipLabels([]string{"volatile"})
	base := mkTarget("__address__", "localhost:9090")
	target := mkTarget("__address__", "localhost:9090", "volatile", "one")
	key := keyFor(base)
	lookups := 0
	c := &fakeCluster{
		peers: allTestPeers, notReady: true,
		lookupMap: map[shard.Key][]peer.Peer{key: {peer1Self}},
		onLookup:  func(got shard.Key) { require.Equal(t, key, got); lookups++ },
	}
	dt := NewDistributedTargets(true, c, []Target{target}, hashing)
	require.Empty(t, dt.LocalTargets())
	require.Equal(t, 1, dt.TargetCount())
	require.Zero(t, lookups)
	c.notReady = false
	dt = NewDistributedTargets(true, c, []Target{target}, hashing)
	require.Equal(t, []Target{target}, dt.LocalTargets())
	require.Equal(t, 1, lookups)

	for _, failKey := range []shard.Key{magicErrorKey, shard.Key(123)} {
		// Both lookup errors and empty results retain the existing local fallback.
		dt = NewDistributedTargets(true, &fakeCluster{peers: allTestPeers}, []Target{target}, TargetHashing{
			Ownership: func(Target) uint64 { return uint64(failKey) },
		})
		require.Equal(t, []Target{target}, dt.LocalTargets())
	}
}
