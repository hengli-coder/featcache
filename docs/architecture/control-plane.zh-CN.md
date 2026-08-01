# 控制面协议（UDS）

控制面通过 Unix Domain Socket 通信，仅用于初始化、元数据查询与版本通知。**所有数据读取走共享内存，不走 UDS。**

> 本文件是 [control-plane.md](control-plane.md)（英文版）的中文翻译。英文版为权威版本。

## 1. 设计原则

- **控制面与数据面分离**：UDS 只用于初始化与元数据
- **数据面零拷贝**：所有数据读取走共享内存
- **协议轻量**：连接即用，无握手开销
- **抽象命名空间**：默认使用 `\x00` 前缀的抽象 socket（无需文件系统）

## 2. OpCode 定义

### 一期实现

| OpCode | 值 | 说明 |
|--------|----|------|
| `OpGetInfo` | `0x01` | 获取内存段元数据（名称、大小、布局） |
| `OpGetStatus` | `0x02` | 获取加载器状态 |

### 二期/三期扩展（预留）

| OpCode | 值 | 说明 |
|--------|----|------|
| `OpWatch` | `0x03` | 监听版本变更通知 |
| `OpPin` | `0x04` | 固定数据到内存（多级存储） |
| `OpPrefetch` | `0x05` | 预取数据到缓存层 |
| `OpEvict` | `0x06` | 淘汰缓存数据 |
| `OpList` | `0x07` | 列出已加载数据集 |
| `OpReload` | `0x08` | 触发重新加载 |

## 3. 请求格式

```
OpCode:   1B
KeyLen:   2B (uint16, big-endian)
Body:     key bytes (KeyLen)
```

| 字段 | 大小 | 说明 |
|------|------|------|
| OpCode | 1B | 操作码 |
| KeyLen | 2B | Key 长度（大端序） |
| Key | KeyLen | 请求体（可变） |

## 4. 响应格式

```
Status:      1B
SegmentName: 64B (固定长度，空字符填充)
SegmentSize: 8B  (uint64, big-endian)
HashOffset:  4B  (uint32, big-endian)
HashCap:     4B  (uint32, big-endian)
DataOffset:  4B  (uint32, big-endian)
GenCounter:  8B  (uint64, big-endian)
```

### Status 值

| 值 | 常量 | 说明 |
|----|------|------|
| `0x00` | `RespOK` | 成功 |
| `0x01` | `RespNotFound` | 未找到 |
| `0x02` | `RespBusy` | 加载中/重载中 |
| `0x03` | `RespError` | 通用错误 |

## 5. 客户端初始化流程

```
推理进程
  │
  ├─ 1. 连接 UDS（抽象命名空间，如 "\x00featcache-<name>"）
  ├─ 2. 发送 GET_INFO 请求
  ├─ 3. 接收响应，获取段元数据（名称、大小、HashOffset、HashCap、DataOffset、GenCounter）
  ├─ 4. mmap 共享内存段
  ├─ 5. 关闭 UDS 连接
  └─ 6. 之后所有查询走共享内存，不再使用 UDS
```

> **当前实现说明**：`Reader.connect` 目前直接打开段；发送真实 GET_INFO 请求是规划的 TODO（见 [roadmap](../design/roadmap.md)）。

## 6. 实现位置

| 内容 | 位置 |
|------|------|
| 协议编解码 | `pkg/featcache/protocol.go` |
| OpCode/StatusCode 定义 | `pkg/featcache/types.go` |
| 服务器 | `pkg/featcache/server.go` |
| 客户端连接 | `pkg/featcache/reader.go` |

## 7. 安全注意事项

- UDS 抽象命名空间仅限同一用户命名空间内访问
- 文件系统 UDS（`/` 前缀路径）默认 `chmod 0777`，生产环境应根据部署调整权限
- 建议通过文件系统权限控制对 `/dev/shm/<segment>` 的访问（默认 0600）
