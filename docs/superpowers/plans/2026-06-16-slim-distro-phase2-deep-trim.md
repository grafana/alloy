# 裁剪版 Alloy Phase 2：深度裁剪（build-tag 排除 converter + 静态集成）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 `slim` build tag 把 `alloy convert` 的 `otelcolconvert`/`staticconvert` 转换器与 `flowcmd` 的静态集成注册排除掉，从而甩掉 AWS/GCP/Azure/vmware/datadog/arrow 等被它们静态拉回的依赖树，把二进制从 321MB 进一步降到 ~220-240MB。默认（不带 tag）构建行为完全不变。

**Architecture:** 沿用仓库已有的 `embedalloyui`/`boringcrypto` build-tag 拆分先例。把 3 处无条件 import（`flowcmd.go` 的 `static/integrations/install`、`converter.go` 的 `otelcolconvert` 和 `staticconvert`）改为按 `!slim`/`slim` tag 分文件提供。`otelcolconvert` 用 `init()` 自注册，只要没人 import 该包，linker 会把整包及其 57 个 OTel SDK 全部摘除——无需改动 `otelcolconvert/` 目录本身。

**Tech Stack:** Go build tags、Grafana Alloy 引擎（alloycli/flowcmd/converter）、Makefile（`GO_TAGS`）。

前置：Phase 1 已完成（all.go + builder-config 裁剪，已在 `slim-collector-distro` 分支）。参考 `docs/superpowers/specs/2026-06-16-slim-collector-distro-design.md`。

---

## 背景事实（已核实）

- `internal/converter/converter.go:51` 的 `Convert` 是 `switch kind` 直接调用 4 个子转换器；`otelcolconvert`/`staticconvert` 在第 9、12 行无条件 import。
- `otelcolconvert` 57 个 `converter_*.go` 各自 import 对应 OTel contrib 组件并在 `init()` 里注册 → AWS/GCP/Azure/datadog 来源。
- `staticconvert/staticconvert.go:25` 与 `flowcmd/flowcmd.go:15` 都 `_ import internal/static/integrations/install` → vmware/azure/gcp exporter 来源。两条路径都要切断。
- `prometheus.exporter.unix`/`self` 等 10 个保留组件**不**依赖 `static/integrations/install`，也不依赖任一 converter（已用 `go list -deps` 验证）→ gate 安全。
- 已有 feature build-tag 先例：`embedalloyui`、`fips/boringcrypto`、`slicelabels`、oracledb `cgo` stub。
- Makefile 会把 `GO_TAGS` 接到 `-tags` 上并自动前置 `gore2regex`。`prometheusconvert`/`promtailconvert` 依赖轻，保持不变。

## 涉及文件

- 修改：`internal/converter/converter.go`（去掉两个重 import，switch 改调本地 helper）
- 新建：`internal/converter/convert_heavy.go`（`//go:build !slim`）
- 新建：`internal/converter/convert_slim.go`（`//go:build slim`）
- 修改：`flowcmd/flowcmd.go`（移除静态集成 blank import）
- 新建：`flowcmd/integrations_full.go`（`//go:build !slim`）

---

## Task 1: 用 build tag 隔离静态集成注册（flowcmd）

**Files:**
- Modify: `flowcmd/flowcmd.go`（删除第 14-15 行的 `// Register integrations` 注释与 blank import）
- Create: `flowcmd/integrations_full.go`

- [ ] **Step 1: 从 `flowcmd/flowcmd.go` 移除静态集成 blank import**

删除 import 块中这两行（当前第 14-15 行）：
```go
	// Register integrations
	_ "github.com/grafana/alloy/internal/static/integrations/install"
```
其余 import 与文件内容保持不变。

- [ ] **Step 2: 新建 `flowcmd/integrations_full.go`**

内容：
```go
//go:build !slim

package flowcmd

// Register grafana-agent static-mode integrations for the full build. Excluded
// from `slim` builds to drop their heavy dependency trees (vmware/azure/gcp
// exporters and friends). slim has no counterpart file: the registry is simply
// left empty, which is safe because slim only runs native Alloy components.
import _ "github.com/grafana/alloy/internal/static/integrations/install"
```

- [ ] **Step 3: 验证默认（full）构建仍编译**

Run: `cd /Users/xiaokun/code/github.com/shalk/alloy && go build ./flowcmd/ && echo FULL_OK`
Expected: `FULL_OK`

- [ ] **Step 4: 验证 slim tag 下 flowcmd 编译且不含 install**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go build -tags slim ./flowcmd/ && echo SLIM_BUILD_OK
go list -tags slim -deps ./flowcmd/ 2>/dev/null | grep -cE "static/integrations/install$"
```
Expected: `SLIM_BUILD_OK`；最后一行 grep 计数为 `0`（slim 下 install 已不在依赖图）。
注：full 下应仍为 1，可选验证 `go list -deps ./flowcmd/ | grep -cE "static/integrations/install$"` 得 `1`。

- [ ] **Step 5: 提交**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
git add flowcmd/flowcmd.go flowcmd/integrations_full.go
git commit -m "feat: Gate static-mode integrations behind !slim build tag

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: 用 build tag 隔离 otelcol/static 转换器（converter）

**Files:**
- Modify: `internal/converter/converter.go`
- Create: `internal/converter/convert_heavy.go`
- Create: `internal/converter/convert_slim.go`

- [ ] **Step 1: 把 `internal/converter/converter.go` 整体替换为**

```go
// Package converter exposes utilities to convert config files from other
// programs to Grafana Alloy configurations.
package converter

import (
	"fmt"

	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/prometheusconvert"
	"github.com/grafana/alloy/internal/converter/internal/promtailconvert"
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
// extraArgs are supported to be passed along to a converter such as enabling
// integrations-next for the static converter. Converters that do not support
// extraArgs will return a critical severity diagnostic if any are passed.
//
// Conversions are made as literally as possible, so the resulting config files
// may be unoptimized (i.e., lacking component reuse). A converted config file
// should just be the starting point rather than the final destination.
//
// Note that not all functionality defined in the input configuration may have
// an equivalent in Grafana Alloy. If the conversion could not complete because
// of mismatched functionality, an error is returned with no resulting config.
// If the conversion completed successfully but generated warnings, an error is
// returned alongside the resulting config.
//
// otelcol and static conversion are only available in non-slim builds; see
// convert_heavy.go / convert_slim.go.
func Convert(in []byte, kind Input, extraArgs []string) ([]byte, diag.Diagnostics) {
	switch kind {
	case InputOtelCol:
		return convertOtelcol(in, extraArgs)
	case InputPrometheus:
		return prometheusconvert.Convert(in, extraArgs)
	case InputPromtail:
		return promtailconvert.Convert(in, extraArgs)
	case InputStatic:
		return convertStatic(in, extraArgs)
	}

	var diags diag.Diagnostics
	diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("unrecognized kind %q given to the config converter", kind))
	return nil, diags
}
```

(去掉了 `otelcolconvert`/`staticconvert` 两个 import 和 `SupportedFormats`；`SupportedFormats` 移到下面两个 tagged 文件。)

- [ ] **Step 2: 新建 `internal/converter/convert_heavy.go`**

```go
//go:build !slim

package converter

import (
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/otelcolconvert"
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

func convertStatic(in []byte, extraArgs []string) ([]byte, diag.Diagnostics) {
	return staticconvert.Convert(in, extraArgs)
}
```

- [ ] **Step 3: 新建 `internal/converter/convert_slim.go`**

```go
//go:build slim

package converter

import (
	"fmt"

	"github.com/grafana/alloy/internal/converter/diag"
)

// SupportedFormats is the reduced set of input formats this slim build can
// convert. otelcol and static conversion are excluded to drop their heavy
// dependency trees (OTel collector components, static-mode integrations).
var SupportedFormats = []string{
	string(InputPrometheus),
	string(InputPromtail),
}

func convertOtelcol(_ []byte, _ []string) ([]byte, diag.Diagnostics) {
	return nil, unsupportedInSlim("otelcol")
}

func convertStatic(_ []byte, _ []string) ([]byte, diag.Diagnostics) {
	return nil, unsupportedInSlim("static")
}

func unsupportedInSlim(format string) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Add(diag.SeverityLevelCritical, fmt.Sprintf("%q conversion is not supported in this slim build of Alloy", format))
	return diags
}
```

- [ ] **Step 4: 验证默认（full）构建编译，且 SupportedFormats 不变**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go build ./internal/converter/ && echo FULL_OK
go vet ./internal/converter/ 2>&1 | head -5
```
Expected: `FULL_OK`，vet 无报错。

- [ ] **Step 5: 验证 slim tag 下编译且不含 otelcolconvert/staticconvert**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go build -tags slim ./internal/converter/ && echo SLIM_BUILD_OK
go list -tags slim -deps ./internal/converter/ 2>/dev/null | grep -cE "otelcolconvert|staticconvert"
```
Expected: `SLIM_BUILD_OK`；grep 计数为 `0`。

- [ ] **Step 6: 提交**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
git add internal/converter/converter.go internal/converter/convert_heavy.go internal/converter/convert_slim.go
git commit -m "feat: Gate otelcol/static converters behind !slim build tag

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: slim release 编译 + 体积测量

**Files:** 产出 `build/alloy`（不提交）

- [ ] **Step 1: 带 slim tag 编译 collector**

先做快速依赖校验（不构建全量）：
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy/collector
for m in "aws/aws-sdk-go-v2/service/s3" "vmware/govmomi" "DataDog/datadog-agent" "googlecloudexporter"; do
  echo "$(go list -tags slim -deps . 2>/dev/null | grep -c "$m")  $m"
done
```
Expected: 这些计数相比 Phase 1（s3≈有、govmomi=29、datadog 多、googlecloud 有）显著下降，理想为 `0`（注：aws-sdk 可能因 prometheus.remote_write 的 sigv4 残留少量，非 service/s3）。

- [ ] **Step 2: 全量 slim release 构建**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
GO_TAGS=slim SKIP_UI_BUILD=1 SKIP_CODE_GENERATION=1 RELEASE_BUILD=1 make alloy 2>&1 | tail -8
echo "exit=$?"
echo "slim-phase2 size (bytes): $(ls -l build/alloy | awk '{print $5}')"
```
Expected: 编译成功（`-tags "gore2regex slim"`）；体积明显小于 Phase 1 的 321,573,138（目标 ~220-240MB / 2.2e8-2.4e8）。
若编译失败，按报错定位是否有未预料的 slim-only 引用问题，报 BLOCKED。

---

## Task 4: slim 二进制功能冒烟测试

**Files:** 复用 `/tmp/slim-test.alloy`（Phase 1 已创建；若不存在则按下方重建）

- [ ] **Step 1: 确保测试配置存在**

Run: `test -f /tmp/slim-test.alloy && echo EXISTS || echo MISSING`
若 `MISSING`，写入 `/tmp/slim-test.alloy`（实例化全部 10 个组件）：
```alloy
prometheus.exporter.self "self" {}

prometheus.exporter.unix "node" {}

discovery.relabel "dr" {
	targets = prometheus.exporter.self.self.targets
	rule {
		target_label = "job"
		replacement  = "integrations/alloy"
	}
}

prometheus.scrape "scrape" {
	targets    = discovery.relabel.dr.output
	forward_to = [prometheus.relabel.filter.receiver]
	job_name   = "integrations/alloy"
}

prometheus.relabel "filter" {
	forward_to = [prometheus.remote_write.default.receiver]
	rule {
		source_labels = ["__name__"]
		regex         = "up"
		action        = "keep"
	}
}

prometheus.remote_write "default" {
	endpoint {
		url = "http://127.0.0.1:9/api/prom/push"
	}
}

local.file_match "demo" {
	path_targets = [{__path__ = "/tmp/slim-does-not-exist.log"}]
}

loki.source.file "demo" {
	targets    = local.file_match.demo.targets
	forward_to = [loki.process.parse.receiver]
}

loki.process "parse" {
	forward_to = [loki.write.default.receiver]
	stage.labels {
		values = {level = "level"}
	}
}

loki.write "default" {
	endpoint {
		url = "http://127.0.0.1:9/loki/api/v1/push"
	}
}
```

- [ ] **Step 2: 运行 8 秒并断言无组件加载错误**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
rm -rf /tmp/slim-data
timeout 8 ./build/alloy run /tmp/slim-test.alloy \
  --server.http.listen-addr=127.0.0.1:43216 \
  --storage.path=/tmp/slim-data \
  > /tmp/slim-run2.log 2>&1; echo "exit=$?"
grep -iE "unknown component|unrecognized component|Failed to build component|could not find component" /tmp/slim-run2.log || echo "NO_COMPONENT_ERRORS"
grep -iE "finished complete graph evaluation" /tmp/slim-run2.log | head -1
```
Expected: `exit=124`；`NO_COMPONENT_ERRORS`；出现 "finished complete graph evaluation"。

- [ ] **Step 3: 验证 `alloy convert otelcol` 在 slim 下给出明确不支持提示（非崩溃）**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
printf 'receivers:\n  nop:\nexporters:\n  nop:\nservice:\n  pipelines:\n    traces:\n      receivers: [nop]\n      exporters: [nop]\n' > /tmp/slim-otel.yaml
./build/alloy convert --source-format=otelcol /tmp/slim-otel.yaml 2>&1 | head -5; echo "rc=$?"
```
Expected: 输出包含 `not supported in this slim build`（来自 convert_slim.go 的 critical diag），进程正常返回非崩溃。若 `--source-format` 校验先行拒绝 otelcol 也可接受（同样说明 slim 下不可用）。

---

## Task 5: 依赖移除复核 + 记录最终结果

**Files:** 追加写入 `/tmp/slim-baseline.txt`

- [ ] **Step 1: 复核重依赖在 slim 构建下的包数**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy/collector
echo "=== full vs slim package counts ==="
for m in "aws/aws-sdk-go-v2" "Azure/azure-sdk" "cloud.google.com/go" "vmware/govmomi" "DataDog" "apache/arrow-go" "opentelemetry-collector-contrib"; do
  full=$(go list -deps . 2>/dev/null | grep -c "$m")
  slim=$(go list -tags slim -deps . 2>/dev/null | grep -c "$m")
  echo "$m: full=$full slim=$slim"
done
```
Expected: 每项 slim 远小于 full（otel-contrib、datadog、govmomi、gcloud、azure 理想接近 0；aws-sdk 可能残留少量 sigv4 相关）。

- [ ] **Step 2: 记录结果**

把"Phase 1 321MB → Phase 2 <size>"、各依赖 full/slim 包数对比，追加写入 `/tmp/slim-baseline.txt`。

---

## 完成标准

- 默认（无 tag）`go build` 全部通过——未破坏正常构建。
- `GO_TAGS=slim` 构建成功，体积显著小于 321MB（目标 ~220-240MB）。
- slim 冒烟测试 10 个组件零加载错误；`alloy convert otelcol` 在 slim 下给出明确不支持提示而非崩溃。
- `go list -tags slim -deps` 确认 otel-contrib/datadog/govmomi 等大幅下降。
- 两次提交（flowcmd gate、converter gate）已在分支上。

## 风险

- `go vet`/编译可能发现 `SupportedFormats` 被其他包以固定下标引用——若有，定位引用方改为按值判断（实现时按报错处理）。
- slim 构建仍会因 `prometheus.remote_write` 的 sigv4 保留一小部分 aws-sdk；这是用户所需组件，接受。
