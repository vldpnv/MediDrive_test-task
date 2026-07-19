## Race Condition 1: Memory Race on Struct Fields (GetStock vs Reserve)
- **Location:** `GetStock` and `Reserve` methods
- **Code:** `return product.Stock` (in GetStock) vs `product.Stock -= quantity` (in Reserve)
- **What happens:** Concurrent read and write to the same memory address (`product.Stock` integer). The Go memory model does not guarantee atomic reads/writes for basic types without synchronization. This triggers the data race detector.
- **Production scenario:** An internal metrics scraper polls `GetStock` every second to update a real-time dashboard, while customer traffic concurrently hits the `Reserve` endpoint to buy that exact product. 
- **Fix approach:** Wrap both the read operation in `GetStock` and the write operation in `Reserve` with a shared `sync.RWMutex` (using `RLock()` for the read and `Lock()` for the write).

## Race Condition 2: TOCTOU (Time-of-Check to Time-of-Use) 
- **Location:** `Reserve` method
- **Code:** 
  ```go
  if product.Stock < quantity { return ErrInsufficientStock }
  // context switch can happen here
  product.Stock -= quantity
  ```
- **What happens:** The check of the inventory and the mutation of the inventory are not atomic. Two goroutines can read the same initial state, pass the validation, and then both apply their deductions, resulting in a negative state.
- **Production scenario:** A high-demand flash sale. A product has 1 item left. User A and User B click "Checkout" at the exact same millisecond. Both goroutines evaluate 1 < 1 as false. Both proceed to deduct 1. The stock goes to -1 (oversell).
- **Fix approach:** The check and the deduction must happen inside the same critical section protected by a single write lock (sync.Mutex or sync.RWMutex.Lock()).


## Race Condition 3: Interleaved Batch Operations (Non-atomic multiple)
- **Location:** `ReserveMultiple` method
- **Code:** 
```go
    for _, item := range items {
        if product.Stock < item.Quantity { return ErrInsufficientStock }
    }
    //Another goroutine can modify stock right here
    for _, item := range items {
        s.products[item.ProductID].Stock -= item.Quantity
    }
```
- **What happens:** Even if the individual reads and writes were thread-safe, the entire batch operation is not isolated. The state of the inventory can mutate between the moment the function finishes checking all items and the moment it begins deducting them.
- **Production scenario:** A  user is buying an iPhone (Stock: 10) and a Phone Case (Stock: 1). The first loop validates both are availabe. Before the second loop starts, another customer buys the last Phone Case. The second loop deducts the iPhone, but the Phone Case stock goes below zero, creating data corruption and a partial transaction failure.
- **Fix approach:** Acquire a global write lock at the start of ReserveMultiple and release it only after both loops have completed, ensuring the entire batch operates as a single atomic transaction.

## Race Condition 4: Useless Local Lock
- **Location:** `SafeReserve` method
- **Code:**
```go
    var mu sync.Mutex
    mu.Lock()
    defer mu.Unlock()
```
- **What happens:** The mutex is declared locally inside the function scope. This means every time a goroutine calls SafeReserve, it allocates a brand-new, independent mutex on its stack. It locks its own personal mutex and proceeds. Zero synchronization occurs between different goroutines.
- **Production scenario:** Developers deploy this fix thinking the endpoint is now thread-safe, allowing upstream services to increase concurrency. The system immediately experiences the exact same TOCTOU overselling bugs as Reserve, but it passes basic, non-concurrent unit tests.
- **Fix approach:** The mutex must be a shared resource. It needs to be a field on the InventoryService struct itself (s.mu sync.RWMutex), initialized once when the service is instantiated.