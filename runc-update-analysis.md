# Runc 1.4.0 Update Analysis

## Current Situation

- **Current runc version in alloy**: v1.2.8 (pinned via replace directive)
- **Target runc version**: v1.4.0
- **Blocking dependency**: cadvisor integration

## Problem

Runc v1.3.0+ moved the `libcontainer/cgroups` packages to a separate repository:
- **Old**: `github.com/opencontainers/runc/libcontainer/cgroups`
- **New**: `github.com/opencontainers/cgroups`

The grafana/cadvisor fork (commit `1f04a91701e2`, used via replace directive) still uses:
- runc v1.1.9 (which has the old libcontainer/cgroups packages)  
- Custom non-singleton API changes (plugins parameter, rawOptions)

## Upstream Status

**Google/cadvisor**:
- Latest: v0.56.2
- Supports: runc v1.4.0 with opencontainers/cgroups
- Missing: Non-singleton API changes needed by alloy

**Grafana/cadvisor fork**:
- Based on: old upstream (pre-v0.50)
- Has: Non-singleton API changes
- Uses: runc v1.1.9 (outdated)
- Last updated: July 2024

## Non-Singleton Changes

The grafana/cadvisor fork includes changes from:
- PR #1: "Refactor cadvisor to allow non-global instantiations"
- PR #2: "feat(cont/raw): non-global opt to not collect root Cgroup stats"

These changes modify `manager.New()` signature:
```go
// Grafana fork
func New(plugins map[string]container.Plugin, ..., rawOptions raw.Options) (Manager, error)

// Upstream
func New(memoryCache *memory.InMemoryCache, sysfs sysfs.SysFs, ...) (Manager, error)
```

## Solution Options

### Option 1: Update grafana/cadvisor Fork (Recommended)
1. Rebase grafana/cadvisor non-singleton changes onto upstream v0.56.2+
2. Test the updated fork
3. Update alloy to use the new fork version
4. Update runc to v1.4.0

**Pros**: Complete solution, maintains non-singleton feature
**Cons**: Requires cadvisor fork maintenance, potential merge conflicts

### Option 2: Upstream Non-Singleton Changes
1. Submit non-singleton changes to upstream cadvisor
2. Wait for acceptance and release
3. Update alloy to use upstream
4. Update runc to v1.4.0

**Pros**: Long-term sustainable solution
**Cons**: Time-consuming, uncertain acceptance

### Option 3: Remove Cadvisor Dependency
1. Make cadvisor integration optional or remove it
2. Update runc to v1.4.0

**Pros**: Unblocks runc update immediately
**Cons**: Loses cadvisor functionality (breaking change)

## Recommendation

**Option 1** is the best path forward:
1. Update grafana/cadvisor fork to be based on upstream commit `cb7b871` or v0.56.2
2. Re-apply non-singleton patches
3. Test with alloy
4. Update runc to v1.4.0

## Next Steps

1. Create a new branch in grafana/cadvisor based on upstream v0.56.2
2. Cherry-pick or rebase non-singleton commits
3. Update dependencies (runc v1.4.0, opencontainers/cgroups)
4. Test build and functionality
5. Create PR to grafana/cadvisor fork's master branch
6. Update alloy's dependency-replacements.yaml to use new cadvisor version
7. Update runc to v1.4.0 in alloy
