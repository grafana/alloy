# slicelabels Build Tag - Complete Changes

This document lists all locations where the `slicelabels` build tag has been added for Prometheus v3.7.1 compatibility.

## Why slicelabels is Required

Prometheus v3.7.1 changed `labels.Labels` from `[]Label` (slice) to `struct`. The `slicelabels` build tag enables backward-compatible slice implementation, avoiding breaking changes in:
- Loki
- walqueue  
- Alloy code

Without this tag, the project will not compile.

## Files Modified

### 1. Makefile
- **Line 122**: Added `GO_TAGS ?= slicelabels` as default
- **Line ~173**: Updated `test-packages` target to include slicelabels in go test command

### 2. .golangci.yml
- **Lines 4-7**: Added `run.build-tags` section with slicelabels
- This ensures golangci-lint uses the correct build tag

### 3. GitHub Workflows

#### .github/workflows/build.yml
- **Line 40**: Linux builds - added to GO_TAGS
- **Line 67**: Linux boringcrypto builds - added to GO_TAGS
- **Line 84**: Mac Intel builds - added to GO_TAGS
- **Line 101**: Mac ARM builds - added to GO_TAGS
- **Line 118**: Windows builds - added to GO_TAGS  
- **Line 144**: FreeBSD builds - added to GO_TAGS

#### .github/workflows/test_pr.yml
- **Line 27**: Added to `make GO_TAGS` parameter

#### .github/workflows/test_full.yml
- **Line 37**: Added GO_TAGS environment variable

#### .github/workflows/test_mac.yml
- **Line 32**: Added to `make GO_TAGS` parameter

#### .github/workflows/test_windows.yml
- **Line 34**: Added to go test `-tags` parameter

#### .github/workflows/fuzz-go.yml
- **Line 110**: Added to go test `-tags` parameter

### 4. Dockerfiles

#### Dockerfile
- **Line 35**: Added slicelabels to GO_TAGS

#### Dockerfile.windows
- **Line 82**: Added slicelabels to GO_TAGS

## Verification

To verify the tag is being used:

```bash
# Check Makefile default
make info | grep GO_TAGS

# Build manually
make alloy

# Test manually  
make test

# Lint
golangci-lint run
```

All should include `slicelabels` in the tags.

## What's NOT Modified

These use the Makefile defaults (GO_FLAGS with GO_TAGS):
- `make integration-test` - uses GO_ENV
- `make test-pyroscope` - uses GO_FLAGS
- `make lint` - uses .golangci.yml configuration
- All other make targets that use $(GO_FLAGS)

## If You Get Build Errors

If you see errors like:
```
cannot range over lset (variable of struct type labels.Labels)
cannot slice labels (variable of struct type labels.Labels)
```

You forgot to add the `slicelabels` build tag. Check:
1. Your go build/test command includes `-tags=slicelabels`
2. Your GO_TAGS environment variable includes `slicelabels`
3. Your .golangci.yml has the build-tags section
