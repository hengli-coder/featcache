# featcache — AI 推理的零拷贝运行时数据缓存

> 一次加载，多进程零拷贝共享，热切换。
> Load once. Share across processes with zero-copy reads. Hot-swap ready.

[![Go Version](https://img.shields.io/github/go-mod/go-version/hengli-coder/featcache)](https://github.com/hengli-coder/featcache)
[![Go Report Card](https://goreportcard.com/badge/github.com/hengli-coder/featcache)](https://goreportcard.com/report/github.com/hengli-coder/featcache)
[![CI](https://github.com/hengli-coder/featcache/actions/workflows/ci.yml/badge.svg)](https://github.com/hengli-coder/featcache/actions/workflows/ci.yml)
[![Release](https://github.com/hengli-coder/featcache/actions/workflows/release.yml/badge.svg)](https://github.com/hengli-coder/featcache/actions/workflows/release.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

> 本文档是 [README.md](README.md)（英文版）的中文翻译。英文版是权威版本（source of truth），如有出入以英文版为准。

---

## 简介

**featcache** 是一个面向 AI 推理场景的零拷贝运行时数据缓存。它解决的核心问题是：

> AI 推理进程启动时，大量静态数据（Embedding、Tokenizer、Feature Dictionary 等）的重复加载导致启动慢、资源浪费。

传统方案下，N 个推理进程各自加载一份 10GB+ 数据。featcache 将数据**加载一次**，写入 POSIX 共享内存段，所有进程直接读取——零拷贝、无锁、无 syscall。

```
┌───────────────────────┐  一次加载   ┌───────────────────────┐
│  Loader (featload)    │ ──────────► │  共享内存段             │
│  • 读取 DataSource     │             │  [Header]              │
│  • 写入共享内存段       │             │  [Hash Index]          │
│  • 构建哈希索引         │             │  [Data Region]         │
│  • 提供 UDS 控制面      │             └──────────┬────────────┘
└───────────────────────┘                        │ mmap
                                                 ▼
                          ┌────────────────────────────────────┐
                          │ 推理进程 1  进程 2  ... 进程 N        │
                          │ 直接读共享内存                        │
                          │ 零拷贝 · 无锁 · 无 syscall           │
                          └────────────────────────────────────┘
```

### 核心特性

- **零拷贝读取** — 客户端直接读共享内存，查询为纯内存访问
- **一次加载，多进程共享** — N 个进程共享同一份数据
- **启动即用** — 推理进程仅需 mmap，启动成本与数据量无关
- **紧凑存储** — append-only 数据区，无内部碎片
- **纯 Go** — 唯一外部依赖是 `golang.org/x/sys`（用于 mmap）
- **热切换**（二期）— 运行时替换数据，不中断服务

### 适用场景

| 场景 | 数据示例 | 典型大小 |
|------|---------|---------|
| 推荐系统 | User/Item Embedding 表 | 10GB~100GB |
| LLM 推理 | Tokenizer 词汇表、BPE 编码 | 1GB~10GB |
| 多模态模型 | 图像/文本特征字典 | 5GB~50GB |
| 广告 CTR 预估 | 稀疏特征字典、Lookup 表 | 10GB~30GB |
| RAG 系统 | 文档 Embedding 库 | 10GB~100GB |
| 搜索引擎 | ANN 索引、倒排表 | 5GB~50GB |

---

## 安装

### 前置条件

- Go 1.25+
- Linux（POSIX 共享内存 + mmap；见 [平台支持](#平台支持)）

### 安装 Loader 守护进程

```bash
go install github.com/hengli-coder/featcache/cmd/featload@latest
```

或从 [Releases](https://github.com/hengli-coder/featcache/releases) 下载预编译二进制。

### 作为库使用

```bash
go get github.com/hengli-coder/featcache
```

---

## 快速开始

### 1. 启动守护进程

`featload` 守护进程创建共享内存段并服务 UDS 控制面。数据加载通过 `Loader` API 完成（`-source` CLI 参数在规划中，见 [roadmap](docs/design/roadmap.md)）。

```bash
# 构建
go build ./cmd/featload

# 启动守护进程（段名 "my-embeddings"，默认 2GB）
featload -name my-embeddings -size 10737418240

# 参数示例
featload -name featcache -size 2147483648 -uds '\x00featcache' -version
```

### 2. 用 Loader API 加载数据

```go
import "github.com/hengli-coder/featcache/pkg/featcache"

loader, err := featcache.NewLoader(featcache.LoaderConfig{
    SegmentName: "my-embeddings",
    SegmentSize: 10 << 30, // 10GB
})
if err != nil { /* 处理错误 */ }
defer loader.Destroy()

if err := loader.Init(10_000_000); err != nil { /* 处理错误 */ } // 预估条目数，预分配哈希表

// 从二进制文件加载
ds := featcache.NewFileDataSource("/data/embeddings.bin")
count, err := loader.Load(ds) // Load 内部会调用 ds.Open()
if err != nil { /* 处理错误 */ }

// 或从内存 Map 加载（测试/演示）
ds2 := featcache.NewMapDataSource(map[string][]byte{"key": []byte("value")})
count, err = loader.Load(ds2)
```

### 3. 在推理进程中读取

```go
// 初始化 Reader——一次 UDS 元数据交互，之后全部走共享内存
reader, err := featcache.NewReader("my-embeddings", "\x00featcache")
if err != nil { /* 处理错误 */ }
defer reader.Close()

// 查询（内存级速度）
embedding, ok := reader.Get([]byte("user_embedding_123"))
if !ok { /* 未命中 */ }

// 批量查询
keys := [][]byte{[]byte("user_1"), []byte("user_2")}
values, results := reader.GetBatch(keys)
```

> **注意**：`Get` 返回的字节切片直接指向共享内存——**禁止修改**。如需修改请先复制。

### 4. 运行演示（Linux）

```bash
go run ./examples/featload-demo
```

演示程序通过 `MapDataSource` 加载小数据集，然后在同一进程内零拷贝读回。

---

## 配置

### featload CLI

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-name` | `featcache` | 共享内存段名称 |
| `-size` | `2GB` | 段大小（字节） |
| `-uds` | `\x00featcache` | UDS 地址（`\x00` 前缀 = 抽象命名空间） |
| `-version` | `false` | 打印版本信息并退出 |

### LoaderConfig

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `SegmentName` | — | 共享内存段名称 |
| `SegmentSize` | `2GB` | 段大小（字节） |
| `LoadFactor` | `0.5` | 哈希表负载因子 (0.0–1.0) |

### 内置数据源

| 数据源 | 格式 |
|--------|------|
| `NewFileDataSource(path)` | 二进制：每条 `[keyLen:4B LE][key][valLen:4B LE][val]` |
| `NewLineDataSource(path)` | 文本：每行一个 `key\tvalue` |
| `NewMapDataSource(map)` | 内存 Map（测试/演示） |

自定义数据源实现 [DataSource](pkg/featcache/datasource.go) 接口即可（数据库、对象存储、流式等）。

---

## 架构

featcache 将**控制面**与**数据面**分离：

- **控制面** — Unix Domain Socket，仅用于初始化和元数据（`GET_INFO` / `GET_STATUS`）
- **数据面** — 共享内存；所有数据读取都在这里完成，绝不走 UDS

```
Loader 启动 → 读取数据源 → 写入共享内存段 → 就绪
                                              ↓
推理进程启动 → UDS 获取元数据 → mmap 段 → 直接查询
                                              ↓
           所有 GET 操作走共享内存，绝不走 UDS
```

文档：

- [架构总览](docs/architecture/overview.md)
- [内存布局](docs/architecture/memory-layout.md)
- [并发模型](docs/architecture/concurrency.md)
- [控制面协议](docs/architecture/control-plane.md)
- [设计文档](docs/design/)
- [架构决策记录 (ADR)](docs/design/ADRs.md)

### 平台支持

仅支持 Linux。代码使用 build tag：

- `//go:build linux` — 真实实现（`/dev/shm` + mmap，经 `golang.org/x/sys/unix`）
- `//go:build !linux` — 桩实现，返回 `ErrNotSupported`

核心逻辑用内存字节切片测试，因此测试可在任意平台运行（包括 macOS）。

---

## 项目结构

```
featcache/
├── cmd/
│   └── featload/           # Loader 守护进程入口
├── pkg/
│   └── featcache/          # 核心库
│       ├── types.go        # Header、HashSlot、常量、OpCode
│       ├── hash.go         # HashKey (hash/maphash)
│       ├── segment.go      # Segment API（平台无关）
│       ├── segment_linux.go# Linux mmap 实现
│       ├── segment_other.go# 非 Linux 桩
│       ├── hashtable.go    # 开放寻址哈希表
│       ├── loader.go       # Loader（写入侧）
│       ├── reader.go       # Reader（零拷贝读取侧）
│       ├── server.go       # UDS 控制面服务器
│       ├── datasource.go   # DataSource 接口 + 内置实现
│       ├── protocol.go     # UDS 二进制协议编解码
│       └── *_test.go       # 测试
├── examples/
│   └── featload-demo/      # 端到端示例
└── docs/                   # 架构 + 设计文档
```

---

## 开发

### 环境要求

- Go 1.25+
- Linux（核心功能）/ macOS（测试、开发）

### 常用命令

```bash
# 构建
make build

# 测试（含竞态检测）
make test

# 覆盖率
make coverage

# Lint
make lint

# 全部检查（fmt、vet、lint、test、coverage、license）
make check
```

或直接使用 Go 命令：

```bash
# 运行测试
go test ./pkg/featcache/ -v -count=1

# 竞态检测
go test ./pkg/featcache/ -v -race -count=1

# 基准测试
go test ./pkg/featcache/ -bench=. -benchmem -count=1

# 覆盖率
go test ./pkg/featcache/ -coverprofile=coverage.out -covermode=atomic -count=1
go tool cover -func=coverage.out
```

### 贡献指南

请阅读 [CONTRIBUTING.md](CONTRIBUTING.md) 了解贡献流程、代码规范、提交要求。

### 使用 AI 编程助手？

请阅读 [AI_CONTRIBUTING.zh-CN.md](AI_CONTRIBUTING.zh-CN.md) 了解 AI 辅助贡献的规范与要求。

---

## 性能

| 指标 | 目标 | 说明 |
|------|------|------|
| 推理进程启动 | < 100ms | 与数据量无关（mmap + 一次元数据查询） |
| 单次查询 | < 100ns | 1–2 次原子读 + hash 比较，无 syscall |
| 批量查询 | N × 单次 | 线性扩展，无额外开销 |
| 内存效率 | 数据量 × ~1.002 | 紧凑存储，索引开销极小 |
| 多进程内存节省 | (N-1) × 数据量 | N 个进程共享一份数据 |

---

## 与其他方案的对比

| 方案 | 零拷贝多进程共享 | 查询延迟 | 热更新 | 10GB+ 优化 | 外部依赖 |
|------|----------------|---------|--------|-----------|---------|
| **featcache** | ✅ | < 100ns | ✅ (二期) | ✅ | 无 |
| Redis | ❌ 网络通信 | ~100μs | ✅ | ❌ | 无 |
| FAISS | ⚠️ mmap 共享 | < 100ns | ❌ | ✅ | C++ |
| Plasma (已废弃) | ✅ | < 100ns | ❌ | ❌ | C++ |
| Safetensors | ❌ 各自 mmap | < 100ns | ❌ | ✅ | Python/C++ |

---

## 路线图

- [ ] **Phase 1（当前）**：核心功能 — Segment、HashTable、Loader、Reader、UDS 控制面、数据源抽象
- [ ] **Phase 2**：热更新 — 双缓冲版本切换、`WATCH_VERSION`、增量更新
- [ ] **Phase 3**：增强 — 多级存储、持久化、监控指标

详见 [docs/design/roadmap.md](docs/design/roadmap.md)。

---

## 安全

发现安全问题？请阅读 [SECURITY.zh-CN.md](SECURITY.zh-CN.md)，**不要**在公开渠道（GitHub Issues 等）披露漏洞。

---

## 行为准则

参与本项目即表示同意 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。

---

## License

[Apache License 2.0](LICENSE)
