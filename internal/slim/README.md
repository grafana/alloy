# slim package

Slim packages are lightweight counterparts of `github.com/grafana/alloy/internal/util` that follow the separation of concerns principle by isolating functionality with minimal dependencies.

## Idea
- Maintain small, focused packages with minimal transitive compilation dependencies
- Allow users to control compilation dependencies explicitly
- Separation of concerns

## Performance Impact
Switching from `util.TestLogger` to `slim/testlog.TestLogger` for a trivial empty test `util.TestLogger(t).Log("hello", "world")`:
- **Compilation time**: 21s → 500ms 
- **Binary size**: 78MB → 4MB
- **Dependencies**: 216 → 2 packages downloaded

## Usage
For example testlog:
```go
// Instead of:
import "github.com/grafana/alloy/internal/util"
logger := util.TestLogger(t)

// Use:
import "github.com/grafana/alloy/internal/slim/testlog"  
logger := testlog.TestLogger(t)
```

## Benefits
- Faster dependency downloads and CI builds
- Reduced compilation times for tests
- Smaller binaries with focused functionality
