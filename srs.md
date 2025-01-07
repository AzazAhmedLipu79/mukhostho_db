### 1. **Basic Functional Requirements**

#### 1.1 **Data Storage**

* **Data Structure**: Use a custom data structure (e.g., `hash table`, `B-tree`, etc.) to store key-value pairs in memory.
* **Types of Data**:
  * **Key-Value Pairs**: Store data as key-value pairs.
  * **Data Types**: Support various data types (strings, integers, etc.) as values.

#### 1.2 **Basic Operations**

* **Set**: Insert or update a key-value pair in the database.
  * Input: `key`, `value`
  * Output: Success/Failure
* **Get**: Retrieve the value associated with a given key.
  * Input: `key`
  * Output: `value` or error if not found.
* **Delete**: Remove a key-value pair from the database.
  * Input: `key`
  * Output: Success/Failure
* **List/Display**: Retrieve all keys or key-value pairs.
  * Output: List of keys or all key-value pairs.

### 2. **Advanced Features**

#### 2.1 **Persistence (Data Durability)**

* **Automatic Backup**: The database should periodically back up the in-memory data to disk to avoid data loss when the server shuts down.
  * Use a **write-ahead log** or **snapshot** mechanism for persistence.
* **On-Demand Backup**: Allow users to trigger manual backups.
* **Restore on Start**: On server restart, the database should restore the state from the last saved snapshot or log.

#### 2.2 **Transactions (Optional)**

* **Atomic Transactions**: Allow transactions to be executed atomically (all-or-nothing).
  * Operations within a transaction should either be fully committed or fully rolled back.

#### 2.3 **Memory Management**

* **Memory Size Limitation**: Optionally set a memory limit for the database. Once this limit is exceeded, either data should be evicted or the system should stop adding new entries.
* **Garbage Collection**: Remove expired or stale data (e.g., in cache implementations).

#### 2.4 **Concurrency (Optional)**

* **Multi-threading**: Support handling of multiple simultaneous requests (e.g., multiple clients accessing the database).
* **Locks**: Implement locking mechanisms to avoid race conditions, if the system will support multi-threading.

### 3. **Persistence Mechanisms**

#### 3.1 **Write-Ahead Log (WAL)**

* **Purpose**: Every write operation (insert, update, delete) should first be logged to a **WAL** before being applied to the in-memory structure.
* **Features**:
  * Logs should be sequentially written and flushed to disk.
  * Logs can be replayed upon restart to restore the database to the last known consistent state.
  * Size of the WAL file should be manageable (e.g., delete old log entries after successful application to memory).

#### 3.2 **Snapshotting**

* **Periodic Snapshots**: Store the state of the database in a snapshot file at regular intervals.
  * Example: Store the entire in-memory data structure to a file (e.g., JSON or binary format) every 10 minutes.
* **On-Demand Snapshots**: Allow the user to trigger a snapshot at any time.
* **Restore from Snapshot**: On server restart, automatically restore data from the last snapshot.

#### 3.3 **Persistence Options**

* **File Format**: Support either plain-text (e.g., JSON, CSV) or binary file formats for snapshots and logs.
* **Backup Directory**: Allow configuration of the directory where backups and logs should be stored.

### 4. **Server Behavior**

#### 4.1 **Shutdown Behavior**

* On server shutdown, the system should automatically persist data by either:
  * Flushing the WAL to disk.
  * Creating a snapshot of the current in-memory data.
* Ensure that the system gracefully handles unexpected shutdowns by relying on the WAL to ensure that no partial transactions are lost.

#### 4.2 **Server Restart Behavior**

* On restart, the database should first check if a snapshot exists. If it does, it should load it into memory.
* If no snapshot exists, it should replay the WAL to restore the database to the last consistent state.

### 5. **Data Recovery Mechanisms**

#### 5.1 **Crash Recovery**

* After an unexpected shutdown (e.g., power failure or crash), the database should:
  * Recover using the WAL or snapshot.
  * Ensure consistency by reapplying or discarding partial transactions from the WAL.

#### 5.2 **Data Consistency**

* Ensure that operations like insert, update, and delete are **atomic**.
* During recovery, make sure that no data corruption or inconsistencies occur, even if part of the data was not flushed during the shutdown.

### 6. **Scalability**

#### 6.1 **Data Storage Scaling**

* Design your in-memory database so it can scale with increasing data.
  * Monitor memory usage and consider evicting old or least-used data (LRU, LFU eviction policies).

#### 6.2 **Distributed Capability (Optional)**

* Implement features that allow the in-memory database to be run across multiple nodes or servers (distributed in-memory database).
  * Implement mechanisms for data replication and synchronization.

### 7. **API Design**

#### 7.1 **Interface**

* Provide simple APIs for interacting with the database, for example:
  * `Set(key, value)`
  * `Get(key)`
  * `Delete(key)`
  * `Backup()`
  * `Restore()`

#### 7.2 **Error Handling**

* Provide appropriate error codes or messages for operations that fail, such as trying to get a non-existent key or exceeding memory limits.

### 8. **Security (Optional)**

#### 8.1 **Encryption**

* Optionally, allow encrypting data before storing it in memory or on disk.
* Ensure that encryption keys are handled securely (e.g., not hardcoded).

### 9. **Performance Requirements**

* The in-memory database should provide low-latency operations for data retrieval and manipulation.
* The performance of insert, update, delete, and get operations should be close to constant time (O(1)) for a hash-based structure.
* For logging or snapshotting, the operations should be efficient and not hinder the in-memory performance significantly.

---

### Example Architecture:

* **Data Layer**: A custom data structure like a **hash map** or **B-tree** to store key-value pairs.
* **Persistence Layer**:
  * **WAL** for logging changes.
  * **Snapshot** mechanism for periodic backups.
* **Cache**: Optional in-memory cache for frequently accessed data.
* **Recovery**: On restart, load from either the snapshot or replay WAL to restore the data.

### Final Notes:

This specification covers the essential features of an in-memory database that can be easily extended with persistence, atomic operations, and scalability features. You can implement this using your own custom data structures (like hash maps or trees) while ensuring that persistence is handled via periodic snapshots and write-ahead logs.
