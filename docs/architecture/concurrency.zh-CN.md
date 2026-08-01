# 并发模型

featcache 的并发模型是**单写多读（Single Writer / Multiple Readers）**：一个 Loader 写入，N 个 Reader 进程并发读取。读取路径完全无锁、无 syscall。

> 本文件是 [concurrency.md](concurrency.md)（英文版）的中文翻译。英文版为权威版本。

## 1. 模型概览

```
┌──────────────┐        ┌─────────────────────┐
│  Loader      │  写入  │  共享内存段           │
│  (唯一写入者) │ ─────► │  [Header]            │
│              │        │  [Hash Slots]        │
│              │        │  [Data Region]       │
└──────────────┘        └──────────┬──────────┘
                                   │ 只读访问
                     ┌─────────────┼─────────────┐
                     ▼             ▼             ▼
              ┌────────────┐ ┌────────────┐ ┌────────────┐
              │ Reader 1   │ │ Reader 2   │ │ Reader N   │
              │ 无锁读取    │ │ 无锁读取    │ │ 无锁读取    │
              └────────────┘ └────────────┘ └────────────┘
```

## 2. 写入者（Loader）

### 2.1 数据区写入

```go
// 1. 计算 chunk 大小并原子推进 data_end
dataOff := atomic.AddInt32(&l.dataEnd, int32(chunkSize)) - int32(chunkSize)

// 2. 写入数据：[keyLen:4B][key][value]
binary.LittleEndian.PutUint32(data[absOff:absOff+4], uint32(len(key)))
copy(data[absOff+4:], key)
copy(data[absOff+4+len(key):], value)

// 3. 插入哈希表（数据先就绪）
ht.Insert(hash, key, relOffset, uint32(len(value)))
```

**写入顺序保证**：数据完整写入数据区后，才标记 hash slot 为 `SlotUsed`。这是读者安全读取的核心。

### 2.2 哈希表 slot 写入

```go
// CAS 抢占空 slot（避免多写入者冲突）
if !atomic.CompareAndSwapUint32(statusPtr, status, SlotUsed) {
    // 抢占失败，探测下一个 slot
    continue
}
// 抢占成功 → 写入 hash/offset/vlen
```

## 3. 读者（Reader）

### 3.1 读取流程

```go
// 原子加载 slot 状态（acquire 语义）
status := atomic.LoadUint32((*uint32)(unsafe.Pointer(&ht.data[off+16])))

// 状态检查
switch slot.Status {
case SlotEmpty:
    return nil, false        // 空 slot：未找到
case SlotUsed:
    // 已就绪 → 读取 hash/offset/vlen → 比较 key → 返回 value
case SlotTomb:
    // 墓碑：继续探测
}
```

**关键点**：

- 读者只做**原子加载**，不做任何写入
- 读者要么看到空 slot（数据未就绪），要么看到完整的已写入数据
- **不存在半写状态**（写入者先写数据、后标记 slot）

### 3.2 为什么无锁？

1. **单写者**：只有一个进程写入，无需写写互斥
2. **原子发布**：slot 标记使用原子 store，读者原子 load
3. **不可变数据**：一期数据写入后不变，读者无冲突
4. **无引用计数**：共享内存生命周期由 Loader 管理

## 4. 原子性保证

### 4.1 内存序

| 操作 | 语义 | 保证 |
|------|------|------|
| 写入数据区 | 普通 store | 先于 slot 标记（程序顺序） |
| 标记 slot | 原子 store (release) | 读者可见完整数据 |
| 读取 slot | 原子 load (acquire) | 看到完整写入 |

Go 的 `atomic` 包在支持的平台上提供合适的硬件屏障，跨架构安全（不仅限于 x86）。

### 4.2 读者并发

- 多个 goroutine 可安全并发调用 `Reader.Get`（无共享可变状态）
- 多个进程并发读取共享内存无冲突（只读）

## 5. 删除与墓碑

一期仅支持逻辑删除（`SlotTomb`）：

```go
// Delete: 原子替换状态为 Tombstone
atomic.StoreUint32(statusPtr, SlotTomb)
```

- 墓碑保留探测序列，避免破坏线性探测的连续性
- 墓碑 slot 可被后续 `Insert` 复用（CAS 抢占）

## 6. 已知限制与二期改进

| 限制 | 说明 | 二期方案 |
|------|------|---------|
| 无原地更新 | 一期只写一次 | 双缓冲版本切换 |
| 无空间回收 | 删除留下墓碑 | 新段整体替换 |
| 单写入者 | 不支持多 Loader | 保持单写者（更简单） |

## 7. 相关代码

| 位置 | 内容 |
|------|------|
| `pkg/featcache/hashtable.go` | CAS 写入、原子读、墓碑 |
| `pkg/featcache/loader.go` | 数据区原子推进 |
| `pkg/featcache/reader.go` | 无锁读取 |
