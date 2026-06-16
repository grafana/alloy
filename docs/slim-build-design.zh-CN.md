# 精简版 Alloy 构建 — 设计文档

本文档描述 **精简版 Alloy 发行版（slim distribution）** 的设计：一个裁剪过的
Grafana Alloy 构建，只保留"指标抓取 + remote_write + node_exporter + Loki 文件日志采集"
部署所需的组件，剥离掉用不到的重依赖子系统。最终二进制约 **59 MB**，相比默认 debug
构建的 **~516 MB** 缩减了约 89%。

> 配套：[English version](./slim-build-design.md)

## 1. 背景与动机

从 `collector/` 构建出的默认 Alloy 二进制非常大（debug ~516 MB，strip 后 ~321 MB），
因为它把所有 OTel Collector 组件、所有原生 Alloy 组件（约 180 个），以及它们牵入的重依赖
（AWS/GCP/Azure SDK、Kubernetes client-go、OTel Collector 框架等）全部编了进去。

对于一个只做 **指标抓取 + remote_write + node_exporter + Loki 文件日志** 的轻量单机部署，
其中绝大部分都是无用负担。本设计在保留该部署所需组件的前提下，把体积压缩了约 89%。

## 2. 成果

| 阶段 | 体积 | 改动 |
|------:|-----:|--------------|
| 基线（debug，完整） | 516 MB | 默认 `make alloy`（未 strip） |
| Phase 1 | 321 MB | 裁剪组件注册表 + `-s -w` strip |
| Phase 2 | 165 MB | `slim` tag gate 掉 converter + 静态集成 |
| **Phase 3** | **59 MB** | `slim` tag 移除 k8s client-go + 云 SDK |

总计缩减：**516 MB → 59 MB（-89%）**。

依赖影响（完整 → 精简，按编译包数）：

| 依赖 | 完整 | 精简 |
|---|---:|---:|
| k8s.io/client-go | 148 | 2 |
| hashicorp/go-discover（含云 SDK） | 全套 | 0 |
| DataDog | 193 | 0 |
| apache/arrow-go | 31 | 0 |
| opentelemetry-collector-contrib | 317 | ~47 |

## 3. 保留的组件（白名单）

精简版恰好注册以下 10 个原生 Alloy 组件：

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

`remotecfg`（Fleet Management 远程配置）是 Alloy 引擎的一部分，依然可用——它不是注册表里的组件。

## 4. 架构：两条独立的裁剪轴

体积缩减来自两个正交的机制。

### 轴 A —— 组件注册表裁剪（本分支上为永久修改）

两个注册表被原地改为只含所需内容：

- **原生组件** —— `internal/component/all/all.go` 从约 180 个 blank import 缩减到上面 10 个。
  Alloy 引擎对"注册哪些组件"没有硬性要求；未注册的组件只会在配置引用时报"unknown component"，
  不影响启动。
- **OTel Collector 组件** —— `collector/builder-config.yaml` 精简为只剩 `alloyengine` 扩展、
  confmap providers，以及一个 `nop` receiver/exporter 占位。OCB 生成的文件
  （`main.go`、`components.go`、`go.mod`、`go.sum`）通过
  `make generate-otel-collector-distro` 重新生成。

这条轴是 `slim-collector-distro` 分支上的永久修改。完整组件集保留在 `main` 分支。

### 轴 B —— `slim` build tag（可切换）

Go build tag `slim` 用于 gate 掉那些在**框架层面**（而非组件注册表）被拉进来的重依赖子系统，
沿用仓库已有的拆文件模式（`embedalloyui`、`boringcrypto`）。每个被 gate 的单元都有一个
`//go:build !slim` 文件（完整实现）和一个 `//go:build slim` 文件（stub/空）。用
`GO_TAGS=slim` 构建即选中 stub。

被 gate 的子系统（目标部署均不使用）：

| 子系统 | 文件 | 为何重 |
|---|---|---|
| OTel feature-gate 注册 | `internal/util/otel_feature_gate*.go` | blank import `k8sattributesprocessor` → OTel k8s processor + openshift client-go |
| 集群 peer 发现 | `internal/service/cluster/discovery/go_discovery*.go`、`peer_discovery.go` | `hashicorp/go-discover` → k8s client-go **以及全部云厂商 SDK** |
| 配置转换器（`alloy convert`） | `internal/converter/convert_{heavy,slim}.go`、`converter.go` | `otelcolconvert`/`staticconvert`/`prometheusconvert`/`promtailconvert` → OTel 组件、静态集成、prometheus k8s SD |
| Prometheus SD install | `flowcmd/flowcmd.go`、`flowcmd/integrations_full.go` | `prometheus/discovery/install` 注册全部 SD（k8s、ec2、azure、gce…） |
| 静态模式集成 | `flowcmd/integrations_full.go` | `static/integrations/install` → vmware/azure/gcp exporter |

**为何两条轴都需要。** 只裁组件注册表（轴 A）后，二进制还停在 ~165 MB，因为 k8s client-go
和云 SDK 是被**框架层代码**（集群、转换器、SD install）拉进来的，而非组件注册表。实测表明，
单独移除任何一个锚点都没用——client-go 有多条独立的引入路径。只有把它们**全部**一起 gate
掉（轴 B，phase 3），才把 client-go 从 148 个包降到 2 个、二进制降到 59 MB。

## 5. 构建命令

| 构建 | 命令 | 体积 |
|---|---|---|
| **精简版**（本分支） | `GO_TAGS=slim SKIP_UI_BUILD=1 SKIP_CODE_GENERATION=1 RELEASE_BUILD=1 make alloy` | ~59 MB |
| **完整版**（从 `main`） | `git checkout main && RELEASE_BUILD=1 make alloy` | ~321 MB |

说明：
- `RELEASE_BUILD=1` 追加 `-ldflags "-s -w"`（去掉符号表 + DWARF）。
- `SKIP_UI_BUILD=1` 跳过 `npm` UI 构建；默认构建不嵌入 UI（无 `embedalloyui` tag），故安全。
- `SKIP_CODE_GENERATION=1` 跳过 OCB 重新生成（已提交）。
- Makefile 会自动前置 `gore2regex`，因此实际 tag 为 `gore2regex slim`。

## 6. 精简版的取舍

精简版有意禁用了目标部署用不到的功能。每一项都优雅降级（明确报错），不会崩溃：

- **无集群 peer 自动发现** —— `--cluster.discover-peers` 返回"slim 构建不支持"错误。
  静态的 `--cluster.join-addresses` 仍可用。
- **无 `alloy convert`** —— 转换 otelcol/static/prometheus/promtail 配置时返回一个 critical
  诊断，提示改用完整版。
- **无 OTel Collector 组件 / `alloy otel`** —— OTel 组件集未编入。

核心部署路径——`alloy run` 跑原生 Prometheus/Loki 组件，包括 remotecfg 下发的配置与自监控
——完全支持。

## 7. 给精简版新增组件

把组件的 blank import 加到 `internal/component/all/all.go` 后重新构建即可。如果新组件牵入了
某个本应排除在精简版之外的重依赖，按第 4 节的拆文件模式用 `slim` tag 把该依赖 gate 掉。

## 8. 验证

精简版二进制通过运行一份实例化全部 10 个组件的配置（`alloy run`）来验证，断言控制器完成图评估
且无组件构建错误。依赖移除用 `go list -tags "gore2regex slim" -deps` 核对（client-go ≤ 2、
go-discover 0）。同时确认每个被 gate 的包在默认（无 tag）构建下都能编译，确保精简版的改动
不会破坏完整发行版。

## 9. 文件地图

```
internal/component/all/all.go                         # 轴 A：原生组件注册表（10 个）
collector/builder-config.yaml                         # 轴 A：OTel 组件注册表（最小化）
internal/util/otel_feature_gate{,_full,_slim}.go      # 轴 B：OTel feature gate
internal/service/cluster/discovery/
    peer_discovery.go                                 # 轴 B：与 tag 无关的分发
    go_discovery.go        (//go:build !slim)         # 轴 B：真实 go-discover
    go_discovery_slim.go   (//go:build slim)          # 轴 B：stub
internal/converter/
    converter.go                                      # 轴 B：与 tag 无关的分发
    convert_heavy.go       (//go:build !slim)         # 轴 B：真实转换器
    convert_slim.go        (//go:build slim)          # 轴 B：stub
flowcmd/flowcmd.go                                    # 轴 B：移除 SD install
flowcmd/integrations_full.go (//go:build !slim)       # 轴 B：SD + 静态集成
```
