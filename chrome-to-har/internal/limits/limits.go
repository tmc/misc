// Package limits provides resource limiting and DoS prevention functionality.
package limits

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// ResourceLimiter manages system resource limits to prevent DoS attacks
type ResourceLimiter struct {
	maxMemoryBytes    uint64
	maxGoroutines     int
	maxConcurrent     int
	activeRequests    int64
	mu                sync.RWMutex
	requestSemaphore  chan struct{}
	verbose           bool
}

// NewResourceLimiter creates a new resource limiter with the specified limits
func NewResourceLimiter(maxMemoryMB uint64, maxGoroutines, maxConcurrent int, verbose bool) *ResourceLimiter {
	rl := &ResourceLimiter{
		maxMemoryBytes:   maxMemoryMB * 1024 * 1024,
		maxGoroutines:    maxGoroutines,
		maxConcurrent:    maxConcurrent,
		requestSemaphore: make(chan struct{}, maxConcurrent),
		verbose:          verbose,
	}
	
	// Fill semaphore with initial capacity
	for i := 0; i < maxConcurrent; i++ {
		rl.requestSemaphore <- struct{}{}
	}
	
	return rl
}

// CheckMemoryUsage verifies current memory usage is within limits
func (rl *ResourceLimiter) CheckMemoryUsage() error {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	if m.Alloc > rl.maxMemoryBytes {
		return fmt.Errorf("memory limit exceeded: %d MB (max: %d MB)", 
			m.Alloc/1024/1024, rl.maxMemoryBytes/1024/1024)
	}
	
	// Also check heap usage
	if m.HeapAlloc > rl.maxMemoryBytes/2 {
		if rl.verbose {
			log.Printf("High heap usage: %d MB (limit: %d MB)", 
				m.HeapAlloc/1024/1024, rl.maxMemoryBytes/1024/1024)
		}
		
		// Force garbage collection to free memory
		runtime.GC()
	}
	
	return nil
}

// CheckGoroutineCount verifies goroutine count is within limits
func (rl *ResourceLimiter) CheckGoroutineCount() error {
	count := runtime.NumGoroutine()
	if count > rl.maxGoroutines {
		return fmt.Errorf("too many goroutines: %d (max: %d)", count, rl.maxGoroutines)
	}
	return nil
}

// AcquireRequest acquires a slot for concurrent request execution
func (rl *ResourceLimiter) AcquireRequest(ctx context.Context) error {
	select {
	case <-rl.requestSemaphore:
		atomic.AddInt64(&rl.activeRequests, 1)
		if rl.verbose {
			log.Printf("Acquired request slot, active: %d", atomic.LoadInt64(&rl.activeRequests))
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReleaseRequest releases a concurrent request slot
func (rl *ResourceLimiter) ReleaseRequest() {
	atomic.AddInt64(&rl.activeRequests, -1)
	rl.requestSemaphore <- struct{}{}
	if rl.verbose {
		log.Printf("Released request slot, active: %d", atomic.LoadInt64(&rl.activeRequests))
	}
}

// GetActiveRequests returns the current number of active requests
func (rl *ResourceLimiter) GetActiveRequests() int64 {
	return atomic.LoadInt64(&rl.activeRequests)
}

// PerformResourceCheck performs a comprehensive resource check
func (rl *ResourceLimiter) PerformResourceCheck() error {
	if err := rl.CheckMemoryUsage(); err != nil {
		return err
	}
	
	if err := rl.CheckGoroutineCount(); err != nil {
		return err
	}
	
	return nil
}

// SetSystemLimits sets OS-level resource limits (requires appropriate permissions)
func SetSystemLimits(maxMemoryMB uint64, maxProcesses uint64, maxOpenFiles uint64) error {
	// Set memory limit (virtual memory)
	if maxMemoryMB > 0 {
		memLimit := &syscall.Rlimit{
			Cur: maxMemoryMB * 1024 * 1024,
			Max: maxMemoryMB * 1024 * 1024,
		}
		
		if err := syscall.Setrlimit(syscall.RLIMIT_AS, memLimit); err != nil {
			return fmt.Errorf("setting memory limit: %w", err)
		}
	}
	
	// Set process limit (platform-specific implementation)
	if err := setProcLimit(maxProcesses); err != nil {
		return fmt.Errorf("setting process limit: %w", err)
	}
	
	// Set file descriptor limit
	if maxOpenFiles > 0 {
		fileLimit := &syscall.Rlimit{
			Cur: maxOpenFiles,
			Max: maxOpenFiles,
		}
		
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, fileLimit); err != nil {
			return fmt.Errorf("setting file descriptor limit: %w", err)
		}
	}
	
	return nil
}

// RateLimiter provides rate limiting functionality
type RateLimiter struct {
	rate         int
	interval     time.Duration
	tokens       chan struct{}
	stop         chan struct{}
	lastRefill   time.Time
	mu           sync.Mutex
	verbose      bool
}

// NewRateLimiter creates a new rate limiter with the specified rate and interval
func NewRateLimiter(rate int, interval time.Duration, verbose bool) *RateLimiter {
	rl := &RateLimiter{
		rate:       rate,
		interval:   interval,
		tokens:     make(chan struct{}, rate),
		stop:       make(chan struct{}),
		lastRefill: time.Now(),
		verbose:    verbose,
	}
	
	// Fill initial tokens
	for i := 0; i < rate; i++ {
		rl.tokens <- struct{}{}
	}
	
	// Start token refill goroutine
	go rl.refillTokens()
	
	return rl
}

// Wait blocks until a token is available or context is cancelled
func (rl *RateLimiter) Wait(ctx context.Context) error {
	select {
	case <-rl.tokens:
		if rl.verbose {
			log.Printf("Rate limiter: token acquired, %d remaining", len(rl.tokens))
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.stop:
		return fmt.Errorf("rate limiter stopped")
	}
}

// TryAcquire attempts to acquire a token without blocking
func (rl *RateLimiter) TryAcquire() bool {
	select {
	case <-rl.tokens:
		if rl.verbose {
			log.Printf("Rate limiter: token acquired (non-blocking), %d remaining", len(rl.tokens))
		}
		return true
	default:
		return false
	}
}

// Stop stops the rate limiter
func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

// refillTokens periodically refills the token bucket
func (rl *RateLimiter) refillTokens() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			elapsed := now.Sub(rl.lastRefill)
			
			// Calculate how many tokens to add based on elapsed time
			tokensToAdd := int(elapsed / rl.interval)
			if tokensToAdd > 0 {
				for i := 0; i < tokensToAdd && i < rl.rate; i++ {
					select {
					case rl.tokens <- struct{}{}:
					default:
						// Channel full, stop adding tokens
						break
					}
				}
				rl.lastRefill = now
				
				if rl.verbose {
					log.Printf("Rate limiter: refilled %d tokens, %d available", 
						tokensToAdd, len(rl.tokens))
				}
			}
			rl.mu.Unlock()
			
		case <-rl.stop:
			return
		}
	}
}

// TimeoutManager manages operation timeouts
type TimeoutManager struct {
	defaultTimeout time.Duration
	maxTimeout     time.Duration
	verbose        bool
}

// NewTimeoutManager creates a new timeout manager
func NewTimeoutManager(defaultTimeout, maxTimeout time.Duration, verbose bool) *TimeoutManager {
	return &TimeoutManager{
		defaultTimeout: defaultTimeout,
		maxTimeout:     maxTimeout,
		verbose:        verbose,
	}
}

// CreateContext creates a context with appropriate timeout
func (tm *TimeoutManager) CreateContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = tm.defaultTimeout
	}
	
	if timeout > tm.maxTimeout {
		timeout = tm.maxTimeout
		if tm.verbose {
			log.Printf("Timeout capped at maximum: %v", tm.maxTimeout)
		}
	}
	
	if tm.verbose {
		log.Printf("Creating context with timeout: %v", timeout)
	}
	
	return context.WithTimeout(parent, timeout)
}

// ConnectionLimiter manages connection limits for different types of connections
type ConnectionLimiter struct {
	maxConnections map[string]int
	activeConns    map[string]int
	mu             sync.RWMutex
	verbose        bool
}

// NewConnectionLimiter creates a new connection limiter
func NewConnectionLimiter(verbose bool) *ConnectionLimiter {
	return &ConnectionLimiter{
		maxConnections: make(map[string]int),
		activeConns:    make(map[string]int),
		verbose:        verbose,
	}
}

// SetLimit sets the maximum number of connections for a given connection type
func (cl *ConnectionLimiter) SetLimit(connType string, maxConns int) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	
	cl.maxConnections[connType] = maxConns
	if cl.verbose {
		log.Printf("Set connection limit for %s: %d", connType, maxConns)
	}
}

// AcquireConnection acquires a connection slot for the specified type
func (cl *ConnectionLimiter) AcquireConnection(connType string) error {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	
	maxConns, exists := cl.maxConnections[connType]
	if !exists {
		return fmt.Errorf("no limit set for connection type: %s", connType)
	}
	
	currentConns := cl.activeConns[connType]
	if currentConns >= maxConns {
		return fmt.Errorf("connection limit reached for %s: %d", connType, maxConns)
	}
	
	cl.activeConns[connType] = currentConns + 1
	if cl.verbose {
		log.Printf("Acquired connection for %s: %d/%d", connType, cl.activeConns[connType], maxConns)
	}
	
	return nil
}

// ReleaseConnection releases a connection slot for the specified type
func (cl *ConnectionLimiter) ReleaseConnection(connType string) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	
	if cl.activeConns[connType] > 0 {
		cl.activeConns[connType]--
		if cl.verbose {
			log.Printf("Released connection for %s: %d/%d", 
				connType, cl.activeConns[connType], cl.maxConnections[connType])
		}
	}
}

// GetActiveConnections returns the current number of active connections for a type
func (cl *ConnectionLimiter) GetActiveConnections(connType string) int {
	cl.mu.RLock()
	defer cl.mu.RUnlock()
	
	return cl.activeConns[connType]
}

// CircuitBreaker implements circuit breaker pattern for fault tolerance
type CircuitBreaker struct {
	maxFailures     int
	resetTimeout    time.Duration
	state           int32 // 0: closed, 1: open, 2: half-open
	failures        int32
	lastFailureTime time.Time
	mu              sync.RWMutex
	verbose         bool
}

const (
	StateClosed   = 0
	StateOpen     = 1
	StateHalfOpen = 2
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration, verbose bool) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
		verbose:      verbose,
	}
}

// Call executes a function through the circuit breaker
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.RLock()
	state := atomic.LoadInt32(&cb.state)
	cb.mu.RUnlock()
	
	if state == StateOpen {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		
		if time.Since(cb.lastFailureTime) > cb.resetTimeout {
			atomic.StoreInt32(&cb.state, StateHalfOpen)
			atomic.StoreInt32(&cb.failures, 0)
			if cb.verbose {
				log.Printf("Circuit breaker: transitioning to half-open state")
			}
		} else {
			return fmt.Errorf("circuit breaker is open")
		}
	}
	
	err := fn()
	
	if err != nil {
		cb.onFailure()
		return err
	}
	
	cb.onSuccess()
	return nil
}

// onSuccess handles successful execution
func (cb *CircuitBreaker) onSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.StoreInt32(&cb.failures, 0)
	if atomic.LoadInt32(&cb.state) == StateHalfOpen {
		atomic.StoreInt32(&cb.state, StateClosed)
		if cb.verbose {
			log.Printf("Circuit breaker: transitioning to closed state")
		}
	}
}

// onFailure handles failed execution
func (cb *CircuitBreaker) onFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	failures := atomic.AddInt32(&cb.failures, 1)
	cb.lastFailureTime = time.Now()
	
	if failures >= int32(cb.maxFailures) {
		atomic.StoreInt32(&cb.state, StateOpen)
		if cb.verbose {
			log.Printf("Circuit breaker: opening due to %d failures", failures)
		}
	}
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() int32 {
	return atomic.LoadInt32(&cb.state)
}

// ProcessMonitor monitors system processes and resource usage
type ProcessMonitor struct {
	maxCPUPercent float64
	maxMemoryMB   uint64
	checkInterval time.Duration
	stop          chan struct{}
	verbose       bool
}

// NewProcessMonitor creates a new process monitor
func NewProcessMonitor(maxCPUPercent float64, maxMemoryMB uint64, checkInterval time.Duration, verbose bool) *ProcessMonitor {
	return &ProcessMonitor{
		maxCPUPercent: maxCPUPercent,
		maxMemoryMB:   maxMemoryMB,
		checkInterval: checkInterval,
		stop:          make(chan struct{}),
		verbose:       verbose,
	}
}

// Start starts the process monitor
func (pm *ProcessMonitor) Start() {
	go pm.monitor()
}

// Stop stops the process monitor
func (pm *ProcessMonitor) Stop() {
	close(pm.stop)
}

// monitor performs periodic resource monitoring
func (pm *ProcessMonitor) monitor() {
	ticker := time.NewTicker(pm.checkInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			pm.checkResources()
		case <-pm.stop:
			return
		}
	}
}

// checkResources checks current resource usage
func (pm *ProcessMonitor) checkResources() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	memoryMB := m.Alloc / 1024 / 1024
	if memoryMB > pm.maxMemoryMB {
		if pm.verbose {
			log.Printf("Memory usage high: %d MB (max: %d MB)", memoryMB, pm.maxMemoryMB)
		}
		runtime.GC() // Force garbage collection
	}
	
	goroutines := runtime.NumGoroutine()
	if pm.verbose {
		log.Printf("Resource check: Memory: %d MB, Goroutines: %d", memoryMB, goroutines)
	}
}