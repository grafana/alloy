package harness

import (
	"flag"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
)

var shardFlag = flag.String("shard", "", "run only tests assigned to shard i/n (0 <= i < n)")

// shardConfig splits test packages across N parallel CI jobs. A package
// runs only when fnv32a(pkgPath) % total == index, keeping packages with
// multiple top-level tests on the same shard.
type shardConfig struct {
	index int // 0 <= index < total
	total int
}

// ValidateShard parses s as "i/n" and returns nil iff n > 0 and 0 <= i < n.
func ValidateShard(s string) error {
	_, err := parseShardString(s)
	return err
}

func parseShardString(s string) (shardConfig, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return shardConfig{}, fmt.Errorf("invalid shard %q, expected i/n", s)
	}
	index, err := strconv.Atoi(parts[0])
	if err != nil {
		return shardConfig{}, fmt.Errorf("invalid shard index in %q: %w", s, err)
	}
	total, err := strconv.Atoi(parts[1])
	if err != nil {
		return shardConfig{}, fmt.Errorf("invalid shard total in %q: %w", s, err)
	}
	if total <= 0 {
		return shardConfig{}, fmt.Errorf("invalid shard total %d", total)
	}
	if index < 0 || index >= total {
		return shardConfig{}, fmt.Errorf("invalid shard index %d for total %d", index, total)
	}
	return shardConfig{index: index, total: total}, nil
}

func parseShard() (shardConfig, error) {
	if *shardFlag == "" {
		return shardConfig{}, nil
	}
	return parseShardString(*shardFlag)
}

// shouldRun reports whether this shard owns the given package path.
func (s shardConfig) shouldRun(key string) bool {
	if s.total == 0 {
		return true
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	return int(hasher.Sum32()%uint32(s.total)) == s.index
}

func shardCheck(t *testing.T, name string) {
	t.Helper()
	shard, err := parseShard()
	if err != nil {
		t.Fatalf("invalid shard flag: %v", err)
	}
	if shard.total == 0 {
		return
	}
	if !shard.shouldRun(name) {
		t.Skipf("skipping %s for shard %d/%d", name, shard.index, shard.total)
	}
}
