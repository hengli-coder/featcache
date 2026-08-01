# 参与贡献（Contributing to featcache）

感谢你对 featcache 的兴趣！我们欢迎各种形式的贡献。提交之前请先阅读本指南。

> **使用 AI 编程助手？** 请同时阅读 [AI_CONTRIBUTING.zh-CN.md](AI_CONTRIBUTING.zh-CN.md) — AI 辅助贡献须与人工代码遵循完全相同的工程标准。
>
> 本文件是 [CONTRIBUTING.md](CONTRIBUTING.md)（英文版）的中文翻译。英文版为权威版本。

## 目录

- [贡献方式](#贡献方式)
- [开发环境](#开发环境)
- [代码规范](#代码规范)
- [分支命名](#分支命名)
- [提交信息](#提交信息)
- [Pull Request 流程](#pull-request-流程)
- [代码评审](#代码评审)
- [测试要求](#测试要求)
- [文档要求](#文档要求)

## 贡献方式

有多种方式参与：

- **报告 Bug**：使用 [Bug 模板](.github/ISSUE_TEMPLATE/bug_report.md) 提交 issue
- **功能建议**：使用 [功能模板](.github/ISSUE_TEMPLATE/feature_request.md)
- **设计提案**：使用 [设计模板](.github/ISSUE_TEMPLATE/design_proposal.md) 和 [docs/design/TEMPLATE.md](docs/design/TEMPLATE.md)
- **提交代码**：Fork → 分支 → PR
- **完善文档**：修正错别字、补充示例、改进说明

## 开发环境

### 前置要求

| 工具 | 版本 | 用途 |
|------|------|------|
| Go | 1.25+ | 编译、测试 |
| golangci-lint | v2.x | 代码质量检查 |
| git | 任意 | 版本控制 |

### 环境搭建

```bash
# 1. Fork 并克隆
git clone git@github.com:<your-username>/featcache.git
cd featcache

# 2. 验证环境
make check    # 或逐个执行
```

### 常用命令

```bash
make build      # 编译
make test       # 测试（含 race）
make coverage   # 覆盖率
make lint       # golangci-lint
make vet        # go vet
make check      # 全部检查
make bench      # 基准测试
```

## 代码规范

### 格式化

- 必须通过 `gofmt`（本项目使用 `gofmt -s` 简化）
- 必须通过 `goimports`（本地包前缀 `github.com/hengli-coder/featcache`）

### Lint

- 必须通过 `golangci-lint run ./...`（0 issues）
- 关键 linter：`govet`、`staticcheck`、`errcheck`、`gosec`、`revive`、`gocritic`

### 编码风格

- 遵循 [Effective Go](https://go.dev/doc/effective_go)
- 导出 API 必须有 godoc 注释（以标识符名开头，句号结尾）
- 错误处理：不吞错误（`_ =` 仅在明确场景），使用 `%w` 包装错误
- 命名：遵循 Go 惯例（驼峰、缩写大写如 `ID`/`URL`）
- 禁止不必要的 `interface{}`（使用明确类型或泛型）

### 平台兼容

本项目使用 build tag 区分平台：

```go
//go:build linux     — 真实共享内存实现
//go:build !linux    — 桩实现（返回 ErrNotSupported）
```

- 修改 Linux 实现时必须同步更新桩实现
- 新增平台相关代码必须保证非 Linux 平台可编译、可测试

## 分支命名

```
<type>/<short-description>

示例：
fix/hash-seed-consistency
feat/hot-swap
docs/architecture-overview
test/loader-coverage
chore/dependabot-config
```

| 前缀 | 用途 |
|------|------|
| `feat/` | 新功能 |
| `fix/` | Bug 修复 |
| `docs/` | 文档 |
| `test/` | 测试 |
| `refactor/` | 重构 |
| `perf/` | 性能 |
| `chore/` | 杂项 |

## 提交信息

遵循 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <description>

[body]

[footer]
```

### Type

| Type | 说明 |
|------|------|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 文档 |
| `test` | 测试 |
| `refactor` | 重构 |
| `perf` | 性能优化 |
| `chore` | 构建/工具/依赖 |
| `ci` | CI 配置 |
| `style` | 格式（不影响逻辑） |

### 示例

```
feat(loader): share hash seed via segment header

Readers now derive the hash seed from the segment header instead of
generating a process-local seed, making hashes consistent across
processes (see ADR-6).

Closes #42
```

### 规则

- 首行 ≤ 72 字符
- 动词用一般现在时祈使句（`add`、`fix`，非 `added`/`fixed`）
- 破坏性变更：在 footer 标注 `BREAKING CHANGE:`

## Pull Request 流程

1. **创建分支**：从最新的 `main` 分支切出
2. **开发**：遵循代码规范、测试要求
3. **本地验证**：提交前必须全部通过
   ```bash
   make check
   ```
4. **推送并创建 PR**：使用 [PR 模板](.github/PULL_REQUEST_TEMPLATE.md)
5. **等待评审**：维护者会在 3 个工作日内评审
6. **处理反馈**：根据评审意见修改，重新推送

### PR 要求

- [ ] 描述变更内容和动机
- [ ] 关联相关 issue（`Closes #xx`）
- [ ] 标注变更类型（feat/fix/...）
- [ ] 列出执行的测试及结果
- [ ] 说明覆盖率影响
- [ ] 标注是否破坏性变更
- [ ] 更新相关文档

### 不需要 PR 的情况

- 修注释错别字（可直接 PR，无需 issue）
- 重构但行为完全不变（仍需完整测试）

## 代码评审

### 评审要求

- 至少 1 名维护者 approve
- CI 全部通过（lint / test / coverage / build / security）
- 覆盖率不得下降（见 [覆盖率策略](#覆盖率策略)）

### 评审关注点

- 正确性：边界条件、错误处理、并发安全
- 性能：不引入热路径开销（读取路径无锁无 syscall）
- 平台兼容：非 Linux 平台可编译可测试
- 文档：行为变化是否同步更新文档

## 测试要求

### 覆盖率策略

- **总覆盖率 ≥ 70%**（CI 强制）
- 新增代码不得降低总覆盖率
- 核心模块（`loader.go`、`hashtable.go`、`reader.go`、`server.go`）覆盖率目标 ≥ 80%

```bash
# 本地检查
make coverage
# 查看函数级覆盖率
go tool cover -func=coverage.out
```

### 测试规范

- 新功能必须附带单元测试
- Bug 修复必须附带回归测试
- 表驱动测试（table-driven tests）优先
- 平台无关逻辑用内存段测试（`newTestSegment`）
- 涉及 UDS 的测试使用短路径（macOS 103B 限制）

### 基准测试

性能敏感代码应提供基准测试：

```go
func BenchmarkHashTableGet(b *testing.B) { ... }
```

## 文档要求

- 行为变化必须同步更新文档
- 新增导出 API 必须更新 godoc 注释
- 重大设计变更使用 [设计模板](docs/design/TEMPLATE.md) 提交设计，并在 [ADRs.md](docs/design/ADRs.md) 登记决策
- 新功能更新 [README.md](README.md) 和 [CHANGELOG.md](CHANGELOG.md)

## 其他

- 行为准则：[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- 安全问题：**不要公开**，请私下报告（见 [SECURITY.zh-CN.md](SECURITY.zh-CN.md)）
- 有任何疑问？开一个 issue 或 discussions
