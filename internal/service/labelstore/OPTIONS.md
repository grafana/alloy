# Mutex Contention Optimization Options

## Current Problem

### Production Metrics (53,998 goroutines)
```
Blocked on labelstore mutex:
- GetOrAddGlobalRefID: 26,070 goroutines (48.27%) ← WRITE-HEAVY
- GetLocalRefID:       23,810 goroutines (44.09%) ← READ-ONLY
────────────────────────────────────────────────────
Total blocked:         ~49,880 goroutines (92.4%)
```

### Key Insights
- **Nearly 50/50 split** between read and write operations
- `GetOrAddGlobalRefID` (48%) needs write capability - creates new IDs
- `GetLocalRefID` (44%) is pure read - lookup only
- Single `sync.Mutex` protecting all maps causes severe bottleneck
- **92.4% of all goroutines** blocked on this single mutex

### Why This Matters for Solution Choice
- **RWMutex is NOT effective** for 50/50 read/write splits
- RWMutex helps when reads are 80%+ of traffic
- With 48% writes, the write lock still serializes ~26,000 goroutines
- **Sharding is the only solution** that addresses both operations equally

---

## Quick Comparison for Real-World Traffic

| Solution | GetOrAddGlobalRefID (48%) | GetLocalRefID (44%) | Total Impact | Risk |
|----------|---------------------------|---------------------|--------------|------|
| **Current (Mutex)** | ❌ 26k blocked | ❌ 24k blocked | 0% (baseline) | - |
| **Option 1 (RWMutex)** | ❌ 26k blocked (no change) | ✅ Concurrent | ~40% | Low |
| **Option 2 (Fine-Grained)** | ❌ 26k on globalMut | ✅ Concurrent | ~50-60% | Medium |
| **Option 3 (Sharding)** | ✅ ~815 per shard | ✅ ~744 per shard | **~96%** | Medium |

**Key Takeaway:** Only sharding addresses the GetOrAddGlobalRefID bottleneck (48% of traffic)

---

## Option 1: Use `sync.RWMutex`

Replace `sync.Mutex` with `sync.RWMutex` for read-write separation.

**Changes:**
- `GetLocalRefID()` → `RLock()` (44% of traffic benefits)
- `GetOrAddGlobalRefID()` → **still needs `Lock()`** for write capability
- `Collect()` → `RLock()`
- All write operations keep `Lock()`

**Pros:**
- ✅ Minimal code changes (~10 lines)
- ✅ No API changes required
- ✅ `GetLocalRefID` calls can run concurrently (44% of traffic)
- ✅ Maintains existing atomicity guarantees
- ✅ Low risk, easy to test

**Cons:**
- ⚠️ **Only helps 44% of traffic** (`GetLocalRefID`)
- ⚠️ `GetOrAddGlobalRefID` (48% of traffic) still needs exclusive lock
- ⚠️ RWMutex has higher overhead than Mutex
- ⚠️ Writer starvation under read load could hurt `GetOrAddGlobalRefID`
- ⚠️ Cannot atomically upgrade RLock→Lock in `GetOrAddGlobalRefID`
- ⚠️ **Still single point of contention** - 26k goroutines compete for write lock

**Expected Impact:** **~40-50% reduction** (optimistic, only if reads don't block writes)
- Best case: 44% of goroutines (GetLocalRefID) can proceed concurrently
- Worst case: Write lock contention from GetOrAddGlobalRefID still blocks everyone

**⚠️ Real-World Concern:** With 48% write traffic, RWMutex may perform *worse* than Mutex due to overhead

**Technical Detail - Why GetOrAddGlobalRefID Can't Use RLock:**
```go
// Current pattern - CANNOT split into RLock/Lock safely
func (s *Service) GetOrAddGlobalRefID(l labels.Labels) uint64 {
    s.mut.Lock()  // Must use write lock from start
    defer s.mut.Unlock()
    
    labelHash := l.Hash()
    globalID, found := s.labelsHashToGlobal[labelHash]
    if found {
        return globalID  // 50% of calls are cache hits
    }
    // 50% of calls create new IDs - NEED write lock
    s.globalRefID++
    s.labelsHashToGlobal[labelHash] = s.globalRefID
    return s.globalRefID
}

// Can't do this - race condition between RUnlock and Lock:
// RLock() → check map → RUnlock() → Lock() → check again → write
// Another goroutine could write between RUnlock and Lock!
```

With 26,070 goroutines calling this, even cache hits must wait for write lock.

---

## Option 2: Fine-Grained Mutexes Per Map

Add separate mutexes for each data structure with atomics where possible.

**Changes:**
```go
type Service struct {
    globalRefID         atomic.Uint64
    
    // Separate locks for different concerns
    globalMut           sync.RWMutex  // protects labelsHashToGlobal
    mappingsMut         sync.RWMutex  // protects mappings map
    stalenessMut        sync.Mutex    // protects staleGlobals
    
    mappings            map[string]*remoteWriteMapping
    labelsHashToGlobal  map[uint64]uint64
    staleGlobals        map[uint64]*staleMarker
}
```

**Analysis with Real-World Traffic:**
- `GetOrAddGlobalRefID` (48%): Uses `globalMut` - still needs exclusive Lock for writes
- `GetLocalRefID` (44%): Uses `mappingsMut.RLock()` - can be concurrent

**Pros:**
- ✅ `GetOrAddGlobalRefID` and `GetLocalRefID` don't block each other (separate locks)
- ✅ `globalRefID` becomes lock-free with atomic
- ✅ Staleness tracking doesn't block the hot path
- ✅ Can use RWMutex for mappings (helps GetLocalRefID)

**Cons:**
- ⚠️ `GetOrAddGlobalRefID` still has 26,070 goroutines competing for `globalMut`
- ⚠️ More complex lock ordering - risk of deadlocks
- ⚠️ Harder to reason about correctness
- ⚠️ Need careful audit of all cross-map operations
- ⚠️ `GetOrAddGlobalRefID` cannot use RLock (needs write for new entries)

**Expected Impact:** **~50-60% reduction**
- GetLocalRefID (44%) becomes concurrent on `mappingsMut`
- GetOrAddGlobalRefID (48%) still bottlenecked on `globalMut`
- Better than single lock, but doesn't solve the write contention problem

---

## Option 3: Sharded LabelStore (Wrapper Pattern) ⭐ BEST FOR 50/50 SPLIT

Wrap existing `Service` in a sharding adapter that distributes load across N independent labelstores.

**Changes:**
```go
type ShardedLabelStore struct {
    shards []*Service  // e.g., 32 or 64 shards
}

func (s *ShardedLabelStore) GetOrAddGlobalRefID(l labels.Labels) uint64 {
    shard := s.shards[l.Hash() % len(s.shards)]
    return shard.GetOrAddGlobalRefID(l)
}

func (s *ShardedLabelStore) GetLocalRefID(componentID string, globalRefID uint64) uint64 {
    // Use globalRefID to route to same shard (globalRefID encodes label hash)
    shard := s.shards[globalRefID % uint64(len(s.shards))]
    return shard.GetLocalRefID(componentID, globalRefID)
}
```

**Pros:**
- ✅ **Solves BOTH bottlenecks equally** (48% + 44% = 92% of traffic)
- ✅ Massive parallelism - N independent locks (26,070 goroutines / 32 shards = ~815 per shard)
- ✅ Near-linear scalability with number of shards
- ✅ No changes to existing Service code
- ✅ Can tune shard count for workload
- ✅ Maintains all existing semantics per-shard
- ✅ Works great for both read and write heavy operations

**Cons:**
- ⚠️ Global ID space becomes fragmented (need shard-aware ID generation)
- ⚠️ Metrics collection (`Collect()`) must aggregate across all shards
- ⚠️ Memory overhead of N independent labelstores
- ⚠️ Stale marker cleanup runs N times
- ⚠️ More complex to debug and monitor
- ⚠️ Need consistent hashing/routing between GetOrAddGlobalRefID and GetLocalRefID

**Expected Impact with Real-World Traffic:**
- **32 shards:** 96% reduction in contention (53,998 goroutines → ~1,687 per shard)
- **64 shards:** 98% reduction in contention (53,998 goroutines → ~844 per shard)
- Both `GetOrAddGlobalRefID` and `GetLocalRefID` benefit equally

**Critical for 50/50 Split:** This is the ONLY option that effectively addresses both operations

---

## Option 4: `sync.Map` for Mappings

Use Go's built-in `sync.Map` for component mappings (read-heavy use case).

**Changes:**
```go
type Service struct {
    // Keep mutex for labelsHashToGlobal
    mut                 sync.Mutex
    mappings            sync.Map  // map[string]*remoteWriteMapping
    // ...
}
```

**Pros:**
- ✅ Lock-free reads for component lookups in `GetLocalRefID()`
- ✅ Built-in, well-tested concurrent map
- ✅ Good for read-heavy workloads
- ✅ Minimal code changes for mapping operations

**Cons:**
- ⚠️ Type safety lost (interface{} values)
- ⚠️ `sync.Map` has overhead for write-heavy workloads
- ⚠️ Still need mutex for other maps
- ⚠️ Range operations (like in `Collect()`) are more complex
- ⚠️ Only solves part of the problem (mappings, not labelsHashToGlobal)

**Expected Impact:** 30-50% reduction in contention (only helps GetLocalRefID lookups)

---

## Option 5: Per-Component Locking

Each remote write component gets its own lock for local mappings.

**Changes:**
```go
type remoteWriteMapping struct {
    RemoteWriteID string
    mu            sync.RWMutex  // per-component lock
    localToGlobal map[uint64]uint64
    globalToLocal map[uint64]uint64
}
```

**Pros:**
- ✅ Components don't block each other
- ✅ Natural isolation boundary (each remote_write is independent)
- ✅ Can use RWMutex per component
- ✅ Relatively simple to implement

**Cons:**
- ⚠️ Still need global lock for `labelsHashToGlobal`
- ⚠️ GetOrAddGlobalRefID remains bottleneck
- ⚠️ More memory overhead (N locks for N components)
- ⚠️ Complex lock coordination between global and component locks

**Expected Impact:** 20-40% reduction (helps with local ref operations only)

---

## Option 6: Lock-Free Hash Map

Use a lock-free concurrent hash map library (e.g., `github.com/cornelk/hashmap`).

**Pros:**
- ✅ True lock-free reads and writes
- ✅ Maximum parallelism
- ✅ No lock contention at all

**Cons:**
- ⚠️ External dependency
- ⚠️ Complex to verify correctness
- ⚠️ May not support all operations atomically
- ⚠️ Harder to debug
- ⚠️ Memory model guarantees may differ

**Expected Impact:** 80-95% reduction, but with higher risk

---

## Recommendation Ranking (Based on Real-World 50/50 Split)

### 🥇 **CRITICAL: Option 3 (Sharded LabelStore)**
**Status: MUST DO - Only effective solution for this workload**

- ✅ **Addresses both bottlenecks** (48% GetOrAddGlobalRefID + 44% GetLocalRefID)
- ✅ Highest performance gain (96-98% reduction with 32-64 shards)
- ✅ Proven pattern for high-contention write-heavy scenarios
- ✅ Can be implemented as wrapper (non-breaking)
- ⚠️ Requires careful implementation (~1 week)

**Why it's critical:** With 26,070 goroutines blocked on writes, no RWMutex solution can help

---

### 🥈 **Intermediate: Option 2 (Fine-Grained Locks)**
**Status: Moderate improvement, but doesn't solve write contention**

- ✅ Helps separate concerns (~50-60% reduction)
- ✅ Can be stepping stone to sharding
- ⚠️ GetOrAddGlobalRefID still bottlenecked (26k goroutines on one lock)
- ⚠️ Higher complexity, risk of deadlocks

**Use case:** If you need time to design sharding, do this first

---

### 🥉 **Limited: Option 1 (RWMutex)**
**Status: ⚠️ NOT RECOMMENDED for 50/50 workload**

- ⚠️ **Only helps 44% of traffic** (GetLocalRefID)
- ⚠️ **GetOrAddGlobalRefID (48%) gets no benefit** - still needs exclusive Lock
- ⚠️ RWMutex overhead may make it SLOWER than current Mutex
- ⚠️ Writer starvation could hurt GetOrAddGlobalRefID performance

**Expected Impact:** 30-40% at best (pessimistic due to high write ratio)

**Critical insight:** RWMutex is for read-heavy workloads (80%+ reads). Your workload is 48% writes.

---

### ⚠️ **Not Recommended:**
- Option 4 (sync.Map): Only helps 44% of traffic, type safety issues
- Option 5 (Per-Component): Doesn't solve GetOrAddGlobalRefID bottleneck (48% of traffic)
- Option 6 (Lock-Free): Unnecessary complexity and risk

---

## Final Recommendation for 50/50 Write/Read Split

### **Direct to Sharding (Recommended Path)**

**Why skip RWMutex:**
- Your workload is **48% writes** (GetOrAddGlobalRefID) + **44% reads** (GetLocalRefID)
- RWMutex only helps reads - leaves 26,070 goroutines blocked on write lock
- You'd still need sharding afterward - why implement twice?

**Implementation Plan:**

```go
// Week 1: Implement ShardedLabelStore wrapper
type ShardedLabelStore struct {
    shards    []*Service
    numShards uint64
}

// Route by label hash for GetOrAddGlobalRefID
func (s *ShardedLabelStore) GetOrAddGlobalRefID(l labels.Labels) uint64 {
    hash := l.Hash()
    shard := s.shards[hash%s.numShards]
    // Encode shard index in upper bits of globalRefID
    localID := shard.GetOrAddGlobalRefID(l)
    return (hash%s.numShards)<<56 | localID
}

// Extract shard from globalRefID for GetLocalRefID
func (s *ShardedLabelStore) GetLocalRefID(componentID string, globalRefID uint64) uint64 {
    shardIdx := globalRefID >> 56
    shard := s.shards[shardIdx]
    return shard.GetLocalRefID(componentID, globalRefID&0x00FFFFFFFFFFFFFF)
}
```

**Start with:** 32 or 64 shards
- 32 shards: 53,998 → 1,687 goroutines per shard (96% reduction)
- 64 shards: 53,998 → 844 goroutines per shard (98% reduction)

**Expected Result:** 
- GetOrAddGlobalRefID: 26,070 → 815 goroutines per shard (32 shards)
- GetLocalRefID: 23,810 → 744 goroutines per shard (32 shards)
- **Total blocked: 92.4% → 3-5%**

---

## If You Must Do It Incrementally

Only if you can't allocate 1 week for sharding:

**Week 1:** Option 2 (Fine-Grained Locks) for ~50% improvement
**Week 2-3:** Option 3 (Sharding) for full solution

But this means implementing locking logic twice. Better to go straight to sharding.
