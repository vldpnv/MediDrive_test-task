## Q1: Why does declaring the mutex inside `SafeReserve` make it useless?
A mutex synchronizes execution by sharing its internal state across multiple threads or goroutines. Declaring `var mu sync.Mutex` inside the function scope allocates a completely new, zero-valued mutex instance on every single function invocation. Because each calling goroutine operates on its own isolated, local mutex instance, there is no shared state. As a result, mutual exclusion is completely bypassed, and goroutines execute the critical section concurrently without any synchronization.

## Q2: Per-product locks deadlock and prevention
If Goroutine 1 locks Product A and waits for Product B, while Goroutine 2 simultaneously locks Product B and waits for Product A, the system enters a **Deadlock** state. Both goroutines will hang indefinitely.

Honestly, sharding locks per-product sounds great for performance on paper, but it's a massive footgun in practice if you have batch operations. To prevent deadlocks here, you have to enforce strict lock ordering strategy. Before acquiring any locks, the input batch of IDs must be sorted alphabetically (lexicographically). This guarantees that regardless of which goroutine runs, they will always lock resources in the exact same sequence (e.g., Product A is always locked before Product B), completely breaking the circular wait condition.

## Q3: Why early lock release is worse than no locks
This implementation introduces a classic **TOCTOU (Time-of-Check to Time-of-Use)** vulnerability. It is worse than having no locks at all because it provides a false sense of thread safety while introducing **silent data corruption (negative stock)**.

The critical flaw is that the validation phase (checking stock) and the mutation phase (deducting stock) are completely decoupled into two separate critical sections. If the system has 1 unit of stock left, Goroutine 1 can acquire the first lock, verify the stock is sufficient, and release the lock. Before Goroutine 1 can execute the second block, a context switch occurs. Goroutine 2 acquires the first lock, verifies the exact same stock (still 1), and releases the lock. Both goroutines now proceed to the second block, consecutively deducting 1 unit. The inventory drops to -1, causing an oversell bug despite the code being fully covered by locks.

## Q4: Does a clean `-race` run mean the code is completely race-free?
No, a clean run with the `-race` flag does not guarantee that the code is entirely race-free. The Go race detector is a **dynamic analysis tool**, meaning it only flags data races that *actually manifest at runtime* during the specific execution of the test code paths. 

If the test suite does not generate sufficient concurrency contention, if the runtime scheduler executes goroutines sequentially by chance, or if certain code branches are not covered by the tests, the race detector will not find anything. It proves only that no data races occurred during that specific test run under those specific conditions. It cannot prove the absolute absence of potential static data races.