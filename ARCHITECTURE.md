# Mukhostho DB Architecture

## System Overview

Mukhostho DB is a high-performance in-memory database with the following layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                     API Layer                               │
│  (HTTP REST, gRPC, TCP Protocol)                           │
├─────────────────────────────────────────────────────────────┤
│                 Transaction Manager                         │
│  (ACID Properties, Locking, Isolation)                     │
├─────────────────────────────────────────────────────────────┤
│                   Core Engine                               │
│  (CRUD Operations, Concurrency Control)                    │
├─────────────────────────────────────────────────────────────┤
│                 Memory Manager                              │
│  (LRU Eviction, TTL, Garbage Collection)                   │
├─────────────────────────────────────────────────────────────┤
│                   Data Layer                                │
│  (In-Memory Hash Map + Metadata)                           │
├─────────────────────────────────────────────────────────────┤
│                Persistence Layer                            │
│  (WAL Manager + Snapshot Manager)                          │
├─────────────────────────────────────────────────────────────┤
│                 Storage Layer                               │
│  (File System, Backup Management)                          │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Data Structures
- **Primary Store**: Concurrent HashMap (sync.Map in Go)
- **Metadata Store**: Key metadata (TTL, access time, size)
- **LRU Cache**: Doubly-linked list + HashMap for O(1) eviction
- **Transaction Log**: In-memory buffer for WAL entries

### 2. Concurrency Model
- **Read-Write Locks**: Fine-grained locking per key
- **Transaction Isolation**: MVCC (Multi-Version Concurrency Control)
- **Lock-Free Reads**: Optimistic concurrency for read operations
- **Atomic Operations**: CAS (Compare-And-Swap) for counters

### 3. Persistence Strategy
- **WAL Format**: Binary format with checksums
- **Snapshot Format**: JSON + compression (gzip)
- **Recovery Strategy**: Latest snapshot + WAL replay
- **Durability**: fsync() after each WAL write

### 4. Memory Management
- **Eviction Policy**: LRU with O(1) operations
- **Memory Tracking**: Continuous monitoring with alerts
- **TTL Management**: Background goroutine for cleanup
- **Garbage Collection**: Go's GC + manual cleanup

## Performance Characteristics

| Operation | Time Complexity | Space Complexity |
|-----------|----------------|------------------|
| Get       | O(1) average   | O(1)             |
| Set       | O(1) average   | O(1)             |
| Delete    | O(1) average   | O(1)             |
| ListKeys  | O(n)           | O(n)             |
| Eviction  | O(1)           | O(1)             |
| Snapshot  | O(n)           | O(n)             |
| Recovery  | O(n)           | O(n)             |

## Scalability Design
- Horizontal scaling through sharding (future)
- Vertical scaling through memory optimization
- Concurrent client handling (connection pooling)
- Asynchronous persistence operations
