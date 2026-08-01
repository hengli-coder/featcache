# AI 辅助贡献指南

本项目允许贡献者使用 AI 编程助手（如 Claude Code、Copilot、Cursor 等）辅助开发。**AI 生成的代码必须遵循与人工代码完全相同的工程标准。**

> 本文件是 [AI_CONTRIBUTING.md](AI_CONTRIBUTING.md)（英文版）的中文翻译。英文版为权威版本。

## 1. 适用范围

本指南适用于所有使用 AI 辅助的贡献：

- 代码生成（新功能、Bug 修复）
- 重构
- 测试生成
- 文档生成
- 配置/脚本生成

**无论代码由谁生成，最终责任在提交者（contributor）。**

## 2. AI 使用披露（AI Usage Disclosure）

贡献者应在 PR 描述中披露显著使用 AI 的场景（见 [PR 模板](.github/PULL_REQUEST_TEMPLATE.md)）：

```
## AI-assisted contribution

- [x] 使用了 AI 助手（工具：Claude Code）
- 使用范围：
  - 新功能代码生成：loader.go 的 xxx 函数
  - 测试生成：loader_test.go
  - 文档：docs/architecture/overview.md
- 人工审查：所有 AI 生成代码均已人工 review
```

**披露范围建议**：

| 场景 | 是否披露 |
|------|---------|
| 代码生成 | ✅ 必须 |
| 重构 | ✅ 建议 |
| 测试生成 | ✅ 建议 |
| 文档生成 | ✅ 建议 |
| 小幅修改（如 typo） | ❌ 无需 |

## 3. AI 生成代码的要求

AI 生成的代码必须：

1. **遵循项目编码规范** — 见 [CONTRIBUTING.zh-CN.md](CONTRIBUTING.zh-CN.md)（gofmt、golangci-lint、命名、注释）
2. **通过全部现有测试** — `go test ./...`
3. **包含适当的测试** — 新功能必须附带单元测试
4. **不降低测试覆盖率** — 总覆盖率 ≥ 70%
5. **通过全部 linter** — `golangci-lint run ./...`（0 issues）
6. **通过安全检查** — `govulncheck ./...`、gosec
7. **遵循架构指南** — 见 [docs/architecture/](docs/architecture/) 和 [docs/design/](docs/design/)
8. **避免不必要的重构** — 只改目标代码，不做无关重构
9. **维护 API 兼容性** — 破坏性变更需讨论并标注 `BREAKING CHANGE:`

### 禁止事项

- ❌ 绕过测试（如删除测试、加 `//nolint` 掩盖问题）
- ❌ 无测试的新逻辑
- ❌ 与项目架构冲突的设计（如热路径加锁）
- ❌ 引入未审查的第三方依赖
- ❌ 修改 `.golangci.yml` 掩盖告警（合理放宽除外，需说明理由）

## 4. 提交前的强制验证

提交任何 AI 辅助贡献前，**必须**运行以下命令并全部成功：

```bash
# 1. 全部测试（含 race）
go test ./... -count=1 -race

# 2. 覆盖率
go test ./... -coverprofile=coverage.out -covermode=atomic -count=1
bash scripts/check_coverage.sh coverage.out 70

# 3. Lint
golangci-lint run ./...

# 4. 静态检查
go vet ./...
```

**以下情况禁止提交 PR**：

| 情况 | 处理 |
|------|------|
| 测试失败 | 修复后再提交 |
| 覆盖率下降 | 补充测试 |
| Linter 有未解决告警 | 修复 |
| 构建失败 | 修复 |
| 安全检查失败 | 修复 |

## 5. AI 开发工作流

### 编码前（Before coding）

AI 必须：

1. **理解架构文档** — 阅读 [docs/architecture/overview.md](docs/architecture/overview.md) 和 [docs/design/](docs/design/)
2. **审查现有实现模式** — 参考同类型代码的写法
3. **识别相关测试** — 找到受影响模块的测试
4. **理解 API 兼容性要求** — 见 [CHANGELOG.md](CHANGELOG.md) 版本规则

### 编码中（During coding）

AI 必须：

1. **遵循现有项目约定** — 命名、注释、错误处理风格一致
2. **避免不必要的重构** — 最小化 diff
3. **伴随新功能添加测试** — 覆盖正常、异常、边界路径
4. **行为变化时更新文档** — README、docs、godoc

### 提交前（Before submission）

AI 必须：

1. 运行全部测试（见第 4 节）
2. 运行 linter
3. 验证覆盖率
4. **人工 review 变更**（人类在循环中）
5. 生成修改摘要（PR 描述）

## 6. AI 工具与技能治理（.ai/）

项目通过 [.ai/skill-lock.json](.ai/skill-lock.json) 声明允许使用的 AI 技能与工具。AI 助手应：

- 只使用 skill-lock.json 中列出的技能
- 遵循其中声明的验证步骤
- 不使用未经批准的工具或自动化

## 7. 责任与审核

- **提交者责任**：AI 生成的代码视为提交者自己写的代码，负全部责任
- **维护者审核**：维护者按正常标准评审，不因"AI 生成"降低要求
- **披露不实的后果**：隐瞒 AI 使用并导致问题，按正常贡献规范处理

---

*问题或建议？在 [discussions](https://github.com/hengli-coder/featcache/discussions) 提出。*
