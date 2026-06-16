# 裁剪版 Alloy（slim-collector-distro）设计

## 背景与目标

`collector/` 目录编译出的 `alloy` 二进制约 **493MB**（git 中提交的是 516MB，因为是 debug 编译）。
体积来源经符号分析确认：

- `__TEXT`（代码）284MB；`__DWARF`（调试信息）107MB；`__LINKEDIT`（符号表）58MB。
- 调试信息 + 符号表（~150MB）通过 `-s -w`（`RELEASE_BUILD=1`）即可去掉。
- 代码段大头：AWS SDK v2 ~50MB、crypto ~34MB、pingcap/tidb ~8.7MB、arrow-go ~5.9MB、
  vmware/govmomi、gcp compute、envoy、datadog、azure 等。

**用户实际只使用 5 个原生组件**：

- `prometheus.scrape`
- `prometheus.remote_write`
- `prometheus.exporter.unix`
- `loki.source.file`
- `loki.process`

目标：产出一个只保留上述能力的裁剪版二进制，体积尽可能小。

## 关键事实（已核实）

1. 构建链：`collector/main.go` → `collector/components.go`（引入 `alloyengine` 扩展）
   → `flowcmd` → `internal/alloycli` → `internal/component/all/all.go`（blank-import ~180 个原生组件）。
   OTel 组件则来自 `collector/builder-config.yaml`（OCB 生成 `components.go`）。
2. `internal/component/all/all.go` 是**手维护**文件（非生成），含约 180 行组件 import。
   用户需要的 5 个组件均在其中。
3. `internal/component/all/all_test.go` 只**遍历已注册组件**做检查，**不强校验组件全集** →
   裁剪 `all.go` 不会使该测试失败。
4. AWS SDK v2（~50MB）同时被 OTel 组件（awss3 等）与原生组件（`discovery.aws`、`resourcedetection`）
   引用。**必须两个注册表都裁，才能让 linker 摘掉整棵依赖树。**
5. `make alloy` 会先跑 `generate-source-code` → `generate-otel-collector-distro`，
   即从 `builder-config.yaml` 自动重新生成 `main.go`/`components.go`/`go.mod`/`go.sum`
   （除非 `SKIP_CODE_GENERATION=1`）。所以改 `builder-config.yaml` 后直接 `make alloy` 即可。
6. 这是 `shalk/alloy` fork，CI 的"生成物与上游一致"校验仅在推 grafana 上游时才需在意，本定制构建无此约束。

## 方案：原地裁剪两个注册表（已选定）

### 改动 1：`internal/component/all/all.go`

将 ~180 个 blank import 缩减为仅保留 5 个目标组件：

- `internal/component/loki/process`
- `internal/component/loki/source/file`
- `internal/component/prometheus/exporter/unix`
- `internal/component/prometheus/remotewrite`
- `internal/component/prometheus/scrape`

被移除的 import 通过 git 历史保留，便于回退。Alloy 引擎对"必须注册哪些组件"无硬性要求；
未注册的组件只是在用户配置引用时报"unknown component"，不影响启动。

### 改动 2：`collector/builder-config.yaml`

- `extensions`：仅保留 `github.com/grafana/alloy/extension/alloyengine`（Alloy 集成所需）。
- `receivers` / `processors` / `exporters` / `connectors`：清空或缩到最小。
  用户不使用任何 OTel pipeline。若 OCB 不接受空列表，则各保留一个 `nop`（`nopexporter`/`nopreceiver`，体积极小）。
- `providers`：保留全部 5 个（env/file/http/https/yaml），配置加载所需，体积极小。
- `replaces`：保留不动。多余的 replace 指向未被引用的模块时会被 Go 忽略，无害；
  由 `make generate-module-dependencies` 维护同步。

### 改动 3：编译

```bash
RELEASE_BUILD=1 make alloy
```

`make alloy` 自动从修改后的 `builder-config.yaml` 重新生成 collector 代码并构建；
`RELEASE_BUILD=1` 追加 `-ldflags "-s -w"` 去掉调试信息与符号表。

## 预期结果

- 体积：493MB → 预计 **150–200MB 区间**（crypto/runtime/Prometheus 基础库无法移除）。
- 功能：`alloy run` 支持上述 5 个组件构成的 Prometheus 抓取 + remote_write +
  node_exporter + Loki 文件采集链路；其余组件不可用。

## 风险与待验证项（实现时逐一确认）

1. **converter 反向拉回 OTel 依赖**：`alloy convert` 用到的
   `internal/converter/internal/otelcolconvert` 也 import `component/all`，并可能直接引用
   OTel 组件配置。它随 alloycli 编入二进制，**不受 builder-config 裁剪影响**。
   需测量裁剪后 OTel 依赖是否被真正摘除；若仍占大量体积，再评估是否对 converter 做 build-tag 排除（后续迭代，不在本次范围）。
2. **OCB 是否接受空组件列表**：若 `make generate-otel-collector-distro` 报错，回退为保留 `nop` 组件。
3. **`prometheus.remote_write` 的 sigv4 可能仍引入部分 AWS SDK**：属用户所需组件，接受其带来的依赖。
4. **构建失败排查**：若移除 import 后编译报"未使用/缺失包"，定位到具体引用方修正
   （`all.go` 是唯一聚合点，理论上移除是干净的）。

## 验证方式

1. `RELEASE_BUILD=1 make alloy` 成功产出 `build/alloy`。
2. `ls -lh build/alloy` 记录新体积，与 493MB 对比。
3. `go tool nm -size build/alloy | ...` 复核 AWS/GCP/vmware 等依赖树是否已消失。
4. 用一份仅含 5 个组件的 `config.alloy` 跑 `./build/alloy run config.alloy`，确认启动与采集正常。

## 非目标

- 不引入 build tag 可切换机制（用户选择原地修改的 fork 式裁剪）。
- 不在本次处理 converter 的 OTel 依赖排除（视风险项 1 的测量结果再定）。
- 不修改默认全量构建以外的发布/打包流程。
