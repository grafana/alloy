package harness

import (
	"flag"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"testing"
)

var shardFlag = flag.String("shard", "", "run only tests for shard i/n")

type shardConfig struct {
	index int
	total int
}

func parseShard() (shardConfig, error) {
	if *shardFlag == "" {
		return shardConfig{}, nil
	}

	parts := strings.Split(*shardFlag, "/")
	if len(parts) != 2 {
		return shardConfig{}, fmt.Errorf("invalid shard %q, expected i/n", *shardFlag)
	}

	index, err := strconv.Atoi(parts[0])
	if err != nil {
		return shardConfig{}, fmt.Errorf("invalid shard index in %q: %w", *shardFlag, err)
	}
	total, err := strconv.Atoi(parts[1])
	if err != nil {
		return shardConfig{}, fmt.Errorf("invalid shard total in %q: %w", *shardFlag, err)
	}
	if total <= 0 {
		return shardConfig{}, fmt.Errorf("invalid shard total %d", total)
	}
	if index < 0 || index >= total {
		return shardConfig{}, fmt.Errorf("invalid shard index %d for total %d", index, total)
	}

	return shardConfig{index: index, total: total}, nil
}

func (s shardConfig) shouldRun(key string) bool {
	if s.total == 0 {
		return true
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(key))
	return int(hasher.Sum32()%uint32(s.total)) == s.index
}

func SkipShard(t *testing.T) {
	t.Helper()
	if current == nil {
		t.Fatalf("harness is not initialized, use harness.RunTestMain in TestMain")
	}
	if current.Shard.total == 0 {
		return
	}
	if !current.Shard.shouldRun(current.Name) {
		t.Skipf("skipping %s for shard %d/%d", current.Name, current.Shard.index, current.Shard.total)
	}
}
