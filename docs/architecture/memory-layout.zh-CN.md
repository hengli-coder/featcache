# 内存布局

共享内存段是一个连续字节序列，由三个区域组成：Header、哈希表、数据区。

> 本文件是 [memory-layout.md](memory-layout.md)（英文版）的中文翻译。英文版为权威版本。

## 1. 总体布局

```
Offset 0:      ┌─────────────────────────────────────┐
               │  Header (64B)                       │
               │  Magic / Version / Size             │
               │  GenCounter / HashCap               │
               │  HashOffset / DataOffset / DataEnd  │
               ├─────────────────────────────────────┤
HashOffset:    │  索引区域 (Hash Table)               │
               │  开放寻址哈希表，线性探测               │
               │  每个 slot 24B                       │
               │  slot 数 = 2^N（2 的幂）              │
               ├─────────────────────────────────────┤
DataOffset:    │  数据区域 (Data Region)              │
               │  紧凑存储 key + value                │
               │  [keyLen:4B][key][value]            │
               │  append-only，CAS 原子推进 data_end  │
               └─────────────────────────────────────┘
```

## 2. Header（64 字节，一个 cache line）

所有字段为**本机字节序**（x86/ARM 为 little-endian）。

| 偏移 | 大小 | 字段 | 说明 |
|------|------|------|------|
| 0 | 4 | `Magic` | `0x46454154` ("FEAT" LE) |
| 4 | 4 | `Version` | 布局版本号 |
| 8 | 8 | `Size` | 段总大小 |
| 16 | 8 | `GenCounter` | 版本计数器，每次数据变更递增 |
| 24 | 4 | `HashCap` | 哈希表 slot 数（2 的幂） |
| 28 | 4 | `HashOffset` | 哈希表起始偏移 |
| 32 | 4 | `DataOffset` | 数据区起始偏移 |
| 36 | 4 | `DataEnd` | 数据区已用末尾（原子推进） |
| 40 | 4 | `SegmentID` | 段标识 |
| 44 | 4 | `Flags` | 保留 |
| 48 | 16 | `Reserved` | 保留（可用于种子等） |

对应 Go 结构：[types.go](../../pkg/featcache/types.go) 中的 `Header`。

## 3. Hash Slot（24 字节）

```
Offset  Size  Field
0       8     Hash     完整 64-bit hash
8       4     Offset   数据区偏移（相对 DataOffset）
12      4     VLen     值长度
16      4     Status   SlotEmpty(0) / SlotUsed(1) / SlotTomb(2)
20      4     Reserved
```

**设计要点**：存储完整 64-bit hash 而非截断值，大幅减少伪命中后的 key 比较（key 通常较长）。两个 slot 占 48B，落在同一 cache line 内，预取友好。

## 4. 数据区域

```
[数据区域]
  ↓ data_end 原子推进

  Chunk 0:
    [keyLen: uint32][key: keyLen bytes][value: vlen bytes]

  Chunk 1:
    [keyLen: uint32][key: keyLen bytes][value: vlen bytes]

  ...
```

- **append-only**：一次写入，运行期间不变（一期）
- **无内部碎片**：数据紧凑排列，10GB+ 数据量相比 slab 节省 30%+ 空间
- **Value 是不透明字节序列**：调用方定义序列化方式
  - float32 特征向量（128 维 = 512B，768 维 = 3KB）
  - Tokenizer 词汇表（字符串列表）
  - BPE 编码（byte→token 映射）
  - 任意自定义二进制格式

## 5. 布局计算（Loader.Init）

```
slotsNeeded = expectedEntries * 2          // 负载因子 ~50%
hashCap     = NextPow2(slotsNeeded)        // 2 的幂
hashBytes   = hashCap * SlotSize           // 24B/slot
hashOffset  = HeaderSize                   // 64
dataOffset  = Align(hashOffset + hashBytes, 8)
```

## 6. 对齐与一致性保证

- HashOffset 恒为 `HeaderSize` (64)
- DataOffset 按 8 字节对齐
- 写入顺序保证：**数据先写入数据区，再标记 hash slot 为 Used**
- 读者通过原子加载 slot.Status（acquire 语义）看到完整的已写入数据

## 7. 相关代码

| 概念 | 位置 |
|------|------|
| Header 常量与结构 | `pkg/featcache/types.go` |
| Slot 常量与结构 | `pkg/featcache/types.go` |
| 布局计算 | `pkg/featcache/loader.go` (`Init`) |
| 数据写入 | `pkg/featcache/loader.go` (`put`) |
| 哈希表操作 | `pkg/featcache/hashtable.go` |
