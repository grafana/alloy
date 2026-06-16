# 裁剪版 Alloy Phase 3：移除 k8s client-go（→ ~59MB）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** 在 `slim` build tag 下再 gate 掉 4 处用户用不到、却把 k8s client-go（~320 包）和 go-discover 全套云 SDK 拉进来的依赖，使 `GO_TAGS=slim` 产出的二进制从 165MB 降到 ~59MB。默认（无 tag）构建行为完全不变。

**Architecture:** 沿用 `embedalloyui`/phase-2 的 build-tag 拆文件模式。4 处各拆为 `!slim`(完整) / `slim`(裁剪) 两份。已用完整 spike 验证：4 处全 gate 后 client-go 148→2 包、go-discover→0、体积 165MB→58.6MB，且全部 10 个组件正常加载。

**Tech Stack:** Go build tags、Alloy 运行时（util/cluster/converter/flowcmd）。

前置：phase 1、2 已完成（在 `slim-collector-distro` 分支）。spike 已证明可行性与体积。

---

## 已验证的 4 个 client-go 锚点（用户均不需要）

| # | 文件 | 拉入的重依赖 |
|---|---|---|
| 1 | `internal/util/otel_feature_gate.go` 的 2 个 OTel blank import | k8sattributesprocessor + openshift client-go + stanza |
| 2 | `internal/service/cluster/discovery/` 的 go-discover | go-discover/provider/k8s（client-go）+ 全部云 SDK(aws/azure/gce/…) |
| 3 | `internal/converter` 的 prometheusconvert/promtailconvert | prometheus/discovery/kubernetes → client-go |
| 4 | `flowcmd/flowcmd.go` 的 `prometheus/discovery/install` | prometheus 全套 SD（含 kubernetes）→ client-go |

注：用户的 10 个组件（prometheus.scrape/remote_write/relabel/exporter.*、discovery.relabel、loki.*、local.file_match）经 `go list -deps` 验证**均不直接依赖** k8s SD；其中的 client-go 仅来自上述 #2 集群路径。

---

## Task 1: Gate OTel feature-gate blank imports（util）

**Files:**
- Modify: `internal/util/otel_feature_gate.go`
- Create: `internal/util/otel_feature_gate_full.go`
- Create: `internal/util/otel_feature_gate_slim.go`

- [ ] **Step 1: 把 `internal/util/otel_feature_gate.go` 替换为（去掉 blank import 与 otelFeatureGates 变量，保留函数与类型）**

```go
package util

import (
	"fmt"

	"go.opentelemetry.io/collector/featuregate"
)

type gateDetails struct {
	name    string
	enabled bool
}

// Enables a set of feature gates which should always be enabled in Alloy.
func SetupOtelFeatureGates() error {
	return EnableOtelFeatureGates(otelFeatureGates...)
}

// Enables a set of feature gates in Otel's Global Feature Gate Registry.
func EnableOtelFeatureGates(fgts ...gateDetails) error {
	fgReg := featuregate.GlobalRegistry()

	for _, fg := range fgts {
		err := fgReg.Set(fg.name, fg.enabled)
		if err != nil {
			return fmt.Errorf("error setting Otel feature gate: %w", err)
		}
	}

	return nil
}
```

- [ ] **Step 2: 新建 `internal/util/otel_feature_gate_full.go`**

```go
//go:build !slim

package util

import (
	// Registers the "k8sattr.fieldExtractConfigRegex.disallow" feature gate.
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor"
	// Registers the "filelog.allowFileDeletion" feature gate.
	_ "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
)

// otelFeatureGates are enabled by SetupOtelFeatureGates in the full build.
var otelFeatureGates = []gateDetails{
	{
		// This feature gate allows users of the otel filelogreceiver to use the `delete_after_read` setting.
		name:    "filelog.allowFileDeletion",
		enabled: true,
	},
}
```

- [ ] **Step 3: 新建 `internal/util/otel_feature_gate_slim.go`**

```go
//go:build slim

package util

// The slim build includes no OTel collector components, so there are no OTel
// feature gates to register or enable. Keeping this empty avoids pulling the
// OTel processors (and their k8s/openshift client-go trees) into slim builds.
var otelFeatureGates = []gateDetails{}
```

- [ ] **Step 4: 验证两种构建编译**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go build ./internal/util/ && echo FULL_OK
go build -tags slim ./internal/util/ && echo SLIM_OK
echo "slim k8sattr count: $(go list -tags 'gore2regex slim' -deps ./internal/util/ 2>/dev/null | grep -c k8sattributesprocessor)"
```
Expected: `FULL_OK`、`SLIM_OK`、slim k8sattr count = `0`。

- [ ] **Step 5: 提交**
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
git add internal/util/otel_feature_gate.go internal/util/otel_feature_gate_full.go internal/util/otel_feature_gate_slim.go
git commit -m "feat: Gate OTel feature-gate imports behind !slim build tag

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Gate clustering go-discover（cluster）

**Files:**
- Modify: `internal/service/cluster/discovery/peer_discovery.go`
- Modify: `internal/service/cluster/discovery/go_discovery.go` → 加 `//go:build !slim`
- Create: `internal/service/cluster/discovery/go_discovery_slim.go`

- [ ] **Step 1: 改 `peer_discovery.go`** —— 移除 godiscover 依赖，把 `goDiscoverFactory` 字段类型改为 `any`

具体三处编辑（其余不动）：
1. 删除 import：`	godiscover "github.com/hashicorp/go-discover"`
2. 把 Options 里的字段
   ```go
   	// goDiscoverFactory is a function that can be used to create a new discover.Discover instance.
   	// If nil, godiscover.New is used. Used for testing.
   	goDiscoverFactory goDiscoverFactory
   ```
   改为
   ```go
   	// goDiscoverFactory is an optional override used for testing. In the full
   	// build it holds a func(...godiscover.Option) (*godiscover.Discover, error);
   	// it is unused in slim builds. Typed as any to keep this file tag-agnostic.
   	goDiscoverFactory any
   ```
3. 删除类型定义
   ```go
   // goDiscoverFactory is a function that can be used to create a new discover.Discover instance.
   // Matches discover.New signature.
   type goDiscoverFactory func(opts ...godiscover.Option) (*godiscover.Discover, error)
   ```

- [ ] **Step 2: 给 `go_discovery.go` 顶部加 build tag** —— 在文件第一行之前插入：
```go
//go:build !slim

```
并把其中对 `opt.goDiscoverFactory` 的使用改为从 `any` 还原类型。即把：
```go
	factory := opt.goDiscoverFactory
	if factory == nil {
		factory = discover.New
	}
```
改为：
```go
	factory := discover.New
	if opt.goDiscoverFactory != nil {
		factory = opt.goDiscoverFactory.(func(opts ...discover.Option) (*discover.Discover, error))
	}
```
（其余 `newWithGoDiscovery`/`appendPortIfAbsent` 内容不变。）

- [ ] **Step 3: 新建 `internal/service/cluster/discovery/go_discovery_slim.go`**

```go
//go:build slim

package discovery

import "fmt"

// newWithGoDiscovery is stubbed out in slim builds: go-discover (and its
// k8s/cloud provider SDKs) are excluded to keep the binary small. Slim builds
// support static join peers only; the discover-peers option is unavailable.
func newWithGoDiscovery(_ Options) (DiscoverFn, error) {
	return nil, fmt.Errorf("peer discovery via --cluster.discover-peers is not supported in this slim build of Alloy; use --cluster.join-addresses instead")
}
```

- [ ] **Step 4: 验证两种构建编译 + 测试包**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go build ./internal/service/cluster/... && echo FULL_OK
go build -tags slim ./internal/service/cluster/... && echo SLIM_OK
go test ./internal/service/cluster/discovery/ 2>&1 | tail -5
echo "slim go-discover count: $(go list -tags 'gore2regex slim' -deps ./internal/service/cluster/discovery/ 2>/dev/null | grep -c 'hashicorp/go-discover')"
```
Expected: `FULL_OK`、`SLIM_OK`；测试 `ok`（默认/full 构建，go-discover 测试仍跑）；slim go-discover count = `0`。
若 `peer_discovery_test.go` 因字段类型改 `any` 编译失败，把测试中对该字段赋值处保持原样即可（`any` 可接收任意函数值）；如仍报错，定位后最小化修正测试。

- [ ] **Step 5: 提交**
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
git add internal/service/cluster/discovery/
git commit -m "feat: Gate go-discover cluster peer discovery behind !slim build tag

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Gate prometheus/promtail 转换器（converter）

**Files:**
- Modify: `internal/converter/converter.go`
- Modify: `internal/converter/convert_heavy.go`
- Modify: `internal/converter/convert_slim.go`

- [ ] **Step 1: 把 `internal/converter/converter.go` 的 switch 改为全部走 helper**

将文件替换为：
```go
// Package converter exposes utilities to convert config files from other
// programs to Grafana Alloy configurations.
package converter

import (
	"fmt"

	"github.com/grafana/alloy/internal/converter/diag"
)

// Input represents the type of config file being fed into the converter.
type Input string

const (
	// InputOtelCol indicates that the input file is an OpenTelemetry Collector YAML file.
	InputOtelCol Input = "otelcol"
	// InputPrometheus indicates that the input file is a prometheus YAML file.
	InputPrometheus Input = "prometheus"
	// InputPromtail indicates that the input file is a promtail YAML file.
	InputPromtail Input = "promtail"
	// InputStatic indicates that the input file is a grafana agent static YAML file.
	InputStatic Input = "static"
)

// Convert generates a Grafana Alloy config given an input configuration file.
//
// All format-specific conversion is delegated to build-tag-gated helpers
// (see convert_heavy.go / convert_slim.go). slim builds support no conversion
// formats, which keeps their heavy dependency trees out of the binary.
func Convert(in []byte, kind Input, extraArgs []string) ([]byte, diag.Diagnostics) {
	switch kind {
	case InputOtelCol:
		return convertOtelcol(in, extraArgs)
	case InputPrometheus:
		return convertPrometheus(in, extraArgs)
	case InputPromtail:
		return convertPromtail(in, extraArgs)
	case InputStatic:
		return convertStatic(in, extraArgs)
	}

	var diags diag.Diagnostics
	diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("unrecognized kind %q given to the config converter", kind))
	return nil, diags
}
```

- [ ] **Step 2: 把 `internal/converter/convert_heavy.go` 替换为（加上 prometheus/promtail 的真实实现）**

```go
//go:build !slim

package converter

import (
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/converter/internal/promtailconvert"
	"github.com/grafana/alloy/internal/converter/internal/staticconvert"
)

// SupportedFormats is the full set of input formats this build can convert.
var SupportedFormats = []string{
	string(InputOtelCol),
	string(InputPrometheus),
	string(InputPromtail),
	string(InputStatic),
}

func convertOtelcol(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return otelcolconvert.Convert(in, extraArgs)
}

func convertPrometheus(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return prometheusconvert.Convert(in, extraArgs)
}

func convertPromtail(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return promtailconvert.Convert(in, extraArgs)
}

func convertStatic(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return staticconvert.Convert(in, extraArgs)
}
```

- [ ] **Step 3: 把 `internal/converter/convert_slim.go` 替换为（4 个格式全部 stub）**

```go
//go:build slim

package converter

import (
	"fmt"

	"github.com/grafana/alloy/internal/converter/diag"
)

// SupportedFormats is empty in slim builds: config conversion is unavailable so
// that the converter dependency trees (OTel components, static-mode
// integrations, prometheus/promtail k8s service discovery) stay out of the
// binary.
var SupportedFormats = []string{}

func convertOtelcol(_ []byte, _ []string) ([]byte, diag.Diagnostics)    { return nil, unsupportedInSlim("otelcol") }
func convertPrometheus(_ []byte, _ []string) ([]byte, diag.Diagnostics) { return nil, unsupportedInSlim("prometheus") }
func convertPromtail(_ []byte, _ []string) ([]byte, diag.Diagnostics)   { return nil, unsupportedInSlim("promtail") }
func convertStatic(_ []byte, _ []string) ([]byte, diag.Diagnostics)     { return nil, unsupportedInSlim("static") }

func unsupportedInSlim(format string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("%q conversion is not supported in this slim build of Alloy; use a full (non-slim) build to convert config", format))
	return diags
}
```

- [ ] **Step 4: 验证两种构建编译**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go build ./internal/converter/ ./internal/alloycli/ && echo FULL_OK
go build -tags slim ./internal/converter/ ./internal/alloycli/ && echo SLIM_OK
echo "slim convert deps (expect 0): $(go list -tags 'gore2regex slim' -deps ./internal/converter/ 2>/dev/null | grep -cE 'prometheusconvert|promtailconvert|otelcolconvert|staticconvert')"
```
Expected: `FULL_OK`、`SLIM_OK`；最后计数 `0`。

- [ ] **Step 5: 提交**
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
git add internal/converter/converter.go internal/converter/convert_heavy.go internal/converter/convert_slim.go
git commit -m "feat: Gate all config converters behind !slim build tag

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Gate prometheus SD install（flowcmd）

**Files:**
- Modify: `flowcmd/flowcmd.go`
- Modify: `flowcmd/integrations_full.go`

- [ ] **Step 1: 从 `flowcmd/flowcmd.go` 移除 prometheus SD install blank import**

删除这一行（在 import 块内）：
```go
	_ "github.com/prometheus/prometheus/discovery/install"
```
（注意：保留 `_ "github.com/grafana/alloy/internal/loki/promtail/discovery/consulagent"` 不动——它不是 client-go 锚点。）

- [ ] **Step 2: 把该 import 移入 `flowcmd/integrations_full.go`（`!slim`）**

将 `flowcmd/integrations_full.go` 替换为：
```go
//go:build !slim

package flowcmd

import (
	// Register grafana-agent static-mode integrations for the full build.
	// Excluded from slim builds to drop their heavy dependency trees.
	_ "github.com/grafana/alloy/internal/static/integrations/install"

	// Register all Prometheus service-discovery mechanisms (kubernetes, ec2,
	// azure, gce, consul, ...). Excluded from slim builds: it pulls k8s
	// client-go and cloud SDKs, and slim only scrapes static targets.
	_ "github.com/prometheus/prometheus/discovery/install"
)
```

- [ ] **Step 3: 验证两种构建编译 + SD install 在 slim 下消失**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go build ./flowcmd/ && echo FULL_OK
go build -tags slim ./flowcmd/ && echo SLIM_OK
echo "slim prom-discovery-install count: $(go list -tags 'gore2regex slim' -deps ./flowcmd/ 2>/dev/null | grep -cE 'prometheus/discovery/install$')"
```
Expected: `FULL_OK`、`SLIM_OK`；count = `0`。

- [ ] **Step 4: 提交**
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
git add flowcmd/flowcmd.go flowcmd/integrations_full.go
git commit -m "feat: Gate Prometheus SD install behind !slim build tag

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 5: 构建 + 体积/功能/依赖验证

**Files:** 产出 `build/alloy`（不提交）

- [ ] **Step 1: slim release 构建并测量**
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
GO_TAGS=slim SKIP_UI_BUILD=1 SKIP_CODE_GENERATION=1 RELEASE_BUILD=1 make alloy 2>&1 | tail -4
echo "phase3 slim size (bytes): $(ls -l build/alloy | awk '{print $5}')"
echo "phase2 was: 165406690"
```
Expected: 构建成功；体积约 **5.5e7–6.5e7（~55-65MB）**。

- [ ] **Step 2: client-go / 云 SDK 依赖复核**
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy/collector
echo "client-go: $(go list -tags 'gore2regex slim' -deps . 2>/dev/null | grep -c 'k8s.io/client-go')"
echo "go-discover: $(go list -tags 'gore2regex slim' -deps . 2>/dev/null | grep -c 'hashicorp/go-discover')"
```
Expected: client-go ≤ 3（可忽略），go-discover = 0。

- [ ] **Step 3: 功能冒烟（全部 10 组件）**
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
rm -rf /tmp/slim-data
timeout 8 ./build/alloy run /tmp/slim-test.alloy --server.http.listen-addr=127.0.0.1:43218 --storage.path=/tmp/slim-data > /tmp/slim-run-p3.log 2>&1; echo "exit=$?"
grep -iE "unknown component|Failed to build component" /tmp/slim-run-p3.log || echo "NO_COMPONENT_ERRORS"
grep -iE "finished complete graph evaluation" /tmp/slim-run-p3.log | head -1
```
Expected: `exit=124`、`NO_COMPONENT_ERRORS`、出现 graph evaluation。
（`/tmp/slim-test.alloy` 若不存在，按 phase-2 计划 Task 4 的内容重建。）

- [ ] **Step 4: 默认（full）构建未被破坏**
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go build ./internal/util/ ./internal/service/cluster/... ./internal/converter/ ./flowcmd/ ./internal/alloycli/ && echo FULL_BUILD_OK
```
Expected: `FULL_BUILD_OK`。

---

## 完成标准

- `GO_TAGS=slim ... make alloy` 产出 ~55-65MB 二进制，client-go ≤3 包、go-discover=0。
- slim 二进制 10 个组件零加载错误。
- 默认（无 tag）构建全部编译通过——完整版未受影响。
- 4 次提交（util / cluster / converter / flowcmd gate）在分支上。

## 风险

- `peer_discovery_test.go` 可能因 `goDiscoverFactory` 字段改 `any` 需要微调（断言/赋值）——按编译报错最小化修正。
- slim 下 `--cluster.discover-peers` 与 `alloy convert` 返回明确不支持提示，符合预期。
- 残留 2 个 client-go 包是轻量类型引用，不拉重子树，可接受。
