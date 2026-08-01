# Memory Layout

A shared memory segment is a contiguous byte sequence made of three regions: the Header, the hash table, and the data region.

## 1. Overall layout

```
Offset 0:      ┌─────────────────────────────────────┐
               │  Header (64B)                       │
               │  Magic / Version / Size             │
               │  GenCounter / HashCap               │
               │  HashOffset / DataOffset / DataEnd  │
               ├─────────────────────────────────────┤
HashOffset:    │  Index region (Hash Table)          │
               │  Open-addressed, linear probing     │
               │  24B per slot                       │
               │  slot count = 2^N (power of two)    │
               ├─────────────────────────────────────┤
DataOffset:    │  Data region                        │
               │  Compact key + value storage        │
               │  [keyLen:4B][key][value]            │
               │  Append-only; data_end advanced by  │
               │  atomic CAS                         │
               └─────────────────────────────────────┘
```

## 2. Header (64 bytes, one cache line)

All fields use **native byte order** (little-endian on x86/ARM).

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| 0 | 4 | `Magic` | `0x46454154` ("FEAT" LE) |
| 4 | 4 | `Version` | Layout version |
| 8 | 8 | `Size` | Total segment size |
| 16 | 8 | `GenCounter` | Version counter, incremented on every data change |
| 24 | 4 | `HashCap` | Number of hash table slots (power of two) |
| 28 | 4 | `HashOffset` | Byte offset where the hash table starts |
| 32 | 4 | `DataOffset` | Byte offset where the data region starts |
| 36 | 4 | `DataEnd` | Used end of the data region (advanced atomically) |
| 40 | 4 | `SegmentID` | Segment identifier |
| 44 | 4 | `Flags` | Reserved |
| 48 | 16 | `Reserved` | Reserved (could carry a hash seed, etc.) |

Corresponding Go struct: `Header` in [types.go](../../pkg/featcache/types.go).

## 3. Hash Slot (24 bytes)

```
Offset  Size  Field
0       8     Hash     Full 64-bit hash
8       4     Offset   Offset into the data region (relative to DataOffset)
12      4     VLen     Value length
16      4     Status   SlotEmpty(0) / SlotUsed(1) / SlotTomb(2)
20      4     Reserved
```

**Design note**: storing the full 64-bit hash (instead of a truncated value) dramatically reduces key comparisons after false hits (keys are typically long). Two slots occupy 48B — one cache line — which is prefetch-friendly.

## 4. Data region

```
[Data region]
  ↓ data_end advanced atomically

  Chunk 0:
    [keyLen: uint32][key: keyLen bytes][value: vlen bytes]

  Chunk 1:
    [keyLen: uint32][key: keyLen bytes][value: vlen bytes]

  ...
```

- **Append-only**: written once, unchanged during runtime (Phase 1)
- **No internal fragmentation**: data is packed tightly; vs. a slab allocator, ~30%+ savings at 10GB+ scale
- **Values are opaque byte sequences**: callers define the serialization
  - float32 feature vectors (128 dims = 512B, 768 dims = 3KB)
  - tokenizer vocabularies (string lists)
  - BPE encodings (byte → token maps)
  - any custom binary format

## 5. Layout computation (Loader.Init)

```
slotsNeeded = expectedEntries * 2          // ~50% load factor
hashCap     = NextPow2(slotsNeeded)        // power of two
hashBytes   = hashCap * SlotSize           // 24B per slot
hashOffset  = HeaderSize                   // 64
dataOffset  = Align(hashOffset + hashBytes, 8)
```

## 6. Alignment and consistency guarantees

- `HashOffset` is always `HeaderSize` (64)
- `DataOffset` is 8-byte aligned
- Write ordering: **data is written to the data region first, then the hash slot is marked Used**
- Readers load `slot.Status` atomically (acquire semantics) and see the complete written data

## 7. Related code

| Concept | Location |
|---------|----------|
| Header constants and struct | `pkg/featcache/types.go` |
| Slot constants and struct | `pkg/featcache/types.go` |
| Layout computation | `pkg/featcache/loader.go` (`Init`) |
| Data writes | `pkg/featcache/loader.go` (`put`) |
| Hash table operations | `pkg/featcache/hashtable.go` |
