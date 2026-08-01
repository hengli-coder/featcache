# 架构文档

本目录是 featcache 的架构文档，描述系统的整体设计、组件职责、数据流与关键设计决策。

> 本目录是 [英文版文档](README.md) 的中文翻译。英文版为权威版本（source of truth）。

## 目录

| 文档 | 说明 |
|------|------|
| [overview.md](overview.md) | 系统总览、角色定义、组件职责、数据流 |
| [memory-layout.md](memory-layout.md) | 共享内存段布局：Header、哈希表、数据区 |
| [concurrency.md](concurrency.md) | 并发模型：单写多读、无锁读取、原子性保证 |
| [control-plane.md](control-plane.md) | UDS 控制面协议与通信模型 |

## 相关文档

- [设计文档](../design/) — 各组件设计提案与 ADR
- [CLAUDE.md](../../CLAUDE.md) — 开发者工作指引

## 阅读顺序

1. [overview.md](overview.md) — 先建立全局认知
2. [memory-layout.md](memory-layout.md) — 理解数据如何存储
3. [concurrency.md](concurrency.md) — 理解并发安全模型
4. [control-plane.md](control-plane.md) — 理解控制面协议

---

*本文档更新于 2026-08。如有变更请同步更新 [CHANGELOG.md](../../CHANGELOG.md)。*
