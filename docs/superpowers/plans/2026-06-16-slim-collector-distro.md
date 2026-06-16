# 裁剪版 Alloy（slim-collector-distro）实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `collector/` 编译出的 alloy 二进制从 ~493MB 裁剪到只保留用户实际使用的 10 个原生组件，去掉全部 OTel 组件及其依赖树（AWS/GCP/Azure/vmware/datadog 等），显著减小体积。

**Architecture:** 原地裁剪两个组件注册表——`internal/component/all/all.go`（原生组件，手维护）缩到 10 个 import；`collector/builder-config.yaml`（OTel 组件，OCB 生成源）清空业务组件、仅留 alloyengine 扩展 + providers + 各一个 nop 占位。重新生成 collector 代码后用 `RELEASE_BUILD=1`（`-s -w` strip）编译。

**Tech Stack:** Go 1.26、OpenTelemetry Collector Builder (OCB) v0.139.0、Makefile 构建、Grafana Alloy 引擎。

参考设计：`docs/superpowers/specs/2026-06-16-slim-collector-distro-design.md`

---

## 保留的 10 个组件（白名单）

| 组件 | import 路径 |
|---|---|
| `prometheus.scrape` | `internal/component/prometheus/scrape` |
| `prometheus.remote_write` | `internal/component/prometheus/remotewrite` |
| `prometheus.relabel` | `internal/component/prometheus/relabel` |
| `prometheus.exporter.unix` | `internal/component/prometheus/exporter/unix` |
| `prometheus.exporter.self` | `internal/component/prometheus/exporter/self` |
| `discovery.relabel` | `internal/component/discovery/relabel` |
| `loki.source.file` | `internal/component/loki/source/file` |
| `loki.process` | `internal/component/loki/process` |
| `loki.write` | `internal/component/loki/write` |
| `local.file_match` | `internal/component/local/file_match` |

## 涉及文件

- 修改：`internal/component/all/all.go`（180 import → 10 import）
- 修改：`collector/builder-config.yaml`（清空业务 OTel 组件）
- 重新生成（勿手改）：`collector/main.go`、`collector/components.go`、`collector/go.mod`、`collector/go.sum`
- 新建（仅测试用，不提交）：`/tmp/slim-test.alloy`

---

## Task 1: 记录基线

**Files:** 无改动（仅测量）

- [ ] **Step 1: 记录当前二进制体积与依赖快照**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
ls -l build/alloy | awk '{print "baseline size (bytes):", $5}' | tee /tmp/slim-baseline.txt
go tool nm -size build/alloy 2>/dev/null | awk '{n=$4; for(i=5;i<=NF;i++) n=n" "$i; if(match(n,/vendor\//)){r=substr(n,RSTART+RLENGTH)}else r=n; split(r,a,"/"); k=(a[1]~/\./)?a[1]"/"a[2]"/"a[3]:a[1]; agg[k]+=$2} END{for(k in agg)print agg[k],k}' | sort -rn | head -25 | tee -a /tmp/slim-baseline.txt
```
Expected: 输出含 `baseline size (bytes): ~5xxxxxxxx` 以及 `aws-sdk-go-v2 ~50MB`、`vmware/govmomi`、`google api/compute` 等条目。这些条目应在裁剪后消失或变小。

- [ ] **Step 2: 确认当前在 slim-collector-distro 分支**

Run: `git branch --show-current`
Expected: `slim-collector-distro`

---

## Task 2: 裁剪 all.go 到 10 个组件

**Files:**
- Modify: `internal/component/all/all.go`（整体替换，原 1-185 行）

- [ ] **Step 1: 用 10 个 import 替换整个文件**

将 `internal/component/all/all.go` 整个文件内容替换为：

```go
// Package all imports all known component packages.
package all

import (
	// Trimmed to the components actually used by this distribution.
	// See docs/superpowers/specs/2026-06-16-slim-collector-distro-design.md
	_ "github.com/grafana/alloy/internal/component/discovery/relabel"        // Import discovery.relabel
	_ "github.com/grafana/alloy/internal/component/local/file_match"         // Import local.file_match
	_ "github.com/grafana/alloy/internal/component/loki/process"             // Import loki.process
	_ "github.com/grafana/alloy/internal/component/loki/source/file"         // Import loki.source.file
	_ "github.com/grafana/alloy/internal/component/loki/write"               // Import loki.write
	_ "github.com/grafana/alloy/internal/component/prometheus/exporter/self" // Import prometheus.exporter.self
	_ "github.com/grafana/alloy/internal/component/prometheus/exporter/unix" // Import prometheus.exporter.unix
	_ "github.com/grafana/alloy/internal/component/prometheus/relabel"       // Import prometheus.relabel
	_ "github.com/grafana/alloy/internal/component/prometheus/remotewrite"   // Import prometheus.remote_write
	_ "github.com/grafana/alloy/internal/component/prometheus/scrape"        // Import prometheus.scrape
)
```

- [ ] **Step 2: gofmt 格式化**

Run: `cd /Users/xiaokun/code/github.com/shalk/alloy && gofmt -w internal/component/all/all.go && echo OK`
Expected: `OK`，无报错。

- [ ] **Step 3: 编译验证根模块通过**

Run: `cd /Users/xiaokun/code/github.com/shalk/alloy && go build ./internal/component/all/ && echo BUILD_OK`
Expected: `BUILD_OK`（此时 collector 尚未重生成，但 all 包本身必须编译通过）。

- [ ] **Step 4: 编译验证 collector 仍可构建**

Run: `cd /Users/xiaokun/code/github.com/shalk/alloy/collector && go build -o /tmp/alloy-check-task2 . && echo COLLECTOR_OK && rm -f /tmp/alloy-check-task2`
Expected: `COLLECTOR_OK`。trim all.go 不应破坏 collector（仍带全部 OTel 组件）。
若报错信息形如 `undefined: <symbol>`，说明有非 all.go 路径引用了被删组件——按报错定位引用方处理后再继续。

- [ ] **Step 5: 提交**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
git add internal/component/all/all.go
git commit -m "feat: Trim component registry to slim distro whitelist

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```
Expected: 提交成功，1 file changed。

---

## Task 3: 裁剪 builder-config.yaml 并重新生成

**Files:**
- Modify: `collector/builder-config.yaml`（第 8-92 行：extensions/exporters/processors/receivers/connectors）
- Regenerate: `collector/main.go`、`collector/components.go`、`collector/go.mod`、`collector/go.sum`

- [ ] **Step 1: 替换 extensions 到 connectors 各段**

把 `collector/builder-config.yaml` 中从 `extensions:`（第 8 行）到 `connectors:` 段结束（第 92 行，`forwardconnector` 那行）的内容，整体替换为：

```yaml
extensions:
  - gomod: github.com/grafana/alloy/extension/alloyengine v0.1.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/nopexporter v0.147.0

processors:
  []

receivers:
  - gomod: go.opentelemetry.io/collector/receiver/nopreceiver v0.147.0

connectors:
  []
```

保持 `dist:`（第 1-6 行）、`providers:`（第 94-99 行）、`replaces:`（第 101-153 行）**不变**。

- [ ] **Step 2: 重新生成 collector 代码**

Run: `cd /Users/xiaokun/code/github.com/shalk/alloy && BUILDER_VERSION=v0.139.0 make generate-otel-collector-distro 2>&1 | tail -20`
Expected: 生成成功，无 error。`collector/components.go` 中不再出现 awss3/googlecloud/vcenter 等组件工厂。

**Fallback（仅当上一步因 `nopreceiver` 无法解析而失败时）：** 把 Step 1 中 `receivers:` 段改为
```yaml
receivers:
  - gomod: go.opentelemetry.io/collector/receiver/otlpreceiver v0.147.0
```
然后重跑本 Step。`otlpreceiver` 已在原 go.sum 中，必可解析。

- [ ] **Step 3: 验证生成结果与编译**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy/collector
grep -c "awss3exporter\|googlecloudexporter\|vcenterreceiver" components.go
go build -o /tmp/alloy-check-task3 . && echo COLLECTOR_OK && rm -f /tmp/alloy-check-task3
```
Expected: 第一行输出 `0`（OTel 业务组件已移除）；最后输出 `COLLECTOR_OK`。

- [ ] **Step 4: 提交**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
git add collector/builder-config.yaml collector/main.go collector/components.go collector/go.mod collector/go.sum
git commit -m "feat: Strip OTel components from collector builder config

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```
Expected: 提交成功，多文件变更。

---

## Task 4: Release 编译并测量体积

**Files:** 产出 `build/alloy`（不提交二进制）

- [ ] **Step 1: 用 strip + skip-ui 编译**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
SKIP_UI_BUILD=1 SKIP_CODE_GENERATION=1 RELEASE_BUILD=1 make alloy 2>&1 | tail -15
```
Expected: 编译成功，生成 `build/alloy`。`SKIP_UI_BUILD=1` 跳过 npm（默认构建不嵌 UI）；`SKIP_CODE_GENERATION=1` 跳过重复生成（Task 3 已生成）；`RELEASE_BUILD=1` 加 `-s -w`。

- [ ] **Step 2: 测量并对比体积**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
echo "slim size (bytes): $(ls -l build/alloy | awk '{print $5}')"
cat /tmp/slim-baseline.txt | head -1
```
Expected: slim size 显著小于 baseline（预期落在 ~150–250MB / 1.5e8–2.5e8 bytes 区间）。若 ≥350MB，进入 Task 6 排查 OTel 依赖是否被反向拖回。

---

## Task 5: 功能冒烟测试

**Files:**
- Create: `/tmp/slim-test.alloy`（临时测试配置，不提交）

- [ ] **Step 1: 写一份实例化全部 10 个组件的测试配置**

写入 `/tmp/slim-test.alloy`：

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

- [ ] **Step 2: 运行 8 秒并捕获日志**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
rm -rf /tmp/slim-data
timeout 8 ./build/alloy run /tmp/slim-test.alloy \
  --server.http.listen-addr=127.0.0.1:43215 \
  --storage.path=/tmp/slim-data \
  > /tmp/slim-run.log 2>&1; echo "exit=$?"
```
Expected: `exit=124`（被 timeout 正常杀掉，说明进程持续运行未因配置错误退出）。若 `exit=1`，说明启动即失败，看日志。

- [ ] **Step 3: 断言无组件加载错误**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
echo "--- errors (应为空) ---"
grep -iE "unknown component|unrecognized component|Failed to build component|could not find component" /tmp/slim-run.log || echo "NO_COMPONENT_ERRORS"
echo "--- 启动监听 (应出现) ---"
grep -iE "now listening|finished complete graph evaluation|starting complete graph" /tmp/slim-run.log | head -1
```
Expected: 第一段输出 `NO_COMPONENT_ERRORS`；第二段出现监听/图评估成功日志。
（remote_write/loki.write 因 `127.0.0.1:9` 不可达会有 push 失败的 warn/error，属预期网络错误，可忽略——只要不是上面那 4 类组件构建错误。）

---

## Task 6: 依赖移除复核（含 converter 风险测量）

**Files:** 无改动（仅测量与记录）

- [ ] **Step 1: 复核重依赖是否消失**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go tool nm -size build/alloy 2>/dev/null | awk '{n=$4; for(i=5;i<=NF;i++) n=n" "$i; if(match(n,/vendor\//)){r=substr(n,RSTART+RLENGTH)}else r=n; split(r,a,"/"); k=(a[1]~/\./)?a[1]"/"a[2]"/"a[3]:a[1]; agg[k]+=$2} END{for(k in agg)print agg[k],k}' | sort -rn | head -25
```
Expected: `aws/aws-sdk-go-v2` 大幅缩小或消失、`vmware/govmomi`/`google api compute`/`datadog`/`Azure` 消失或显著变小。

- [ ] **Step 2: 测量残留 OTel 依赖（converter 反拉回风险）**

Run:
```bash
cd /Users/xiaokun/code/github.com/shalk/alloy
go tool nm -size build/alloy 2>/dev/null | grep -c "opentelemetry-collector-contrib"
```
Expected: 记录该数值。若仍很大（例如 > 数 MB 的 contrib 符号），说明 `internal/converter/internal/otelcolconvert` 把 OTel 组件配置拉了回来——这是设计文档"风险项 1"。本任务只测量并记录，**不在本计划范围内处理**；如需进一步裁剪再开新计划评估对 converter 做 build-tag 排除。

- [ ] **Step 3: 记录最终结果**

把"baseline 体积 → slim 体积"对比、消失的依赖列表、残留 contrib 数值，追加写入 `/tmp/slim-baseline.txt` 备查。

---

## 完成标准

- `build/alloy` 体积显著小于 493MB。
- Task 5 冒烟测试无组件构建错误，10 个组件全部正常加载。
- `git log` 含 all.go 与 builder-config 两次提交，生成文件已同步提交。
- converter 残留 OTel 依赖量已测量并记录。
