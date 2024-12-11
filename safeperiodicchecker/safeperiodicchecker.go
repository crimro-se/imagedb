/*
	 	This package provides a thread-safe means of invoking a function that meets the criteria of
		"shouldn't be called too often" and "it's safe to return a cached value"
		If the "Callable" interface is too strict for your needs (eg, only a single return value and no parameters)
		then simply curry your function (use a closure)
*/
package safeperiodicchecker

import (
	"sync"
	"time"
)

// Callable represents a function that can be called up to once every x time.Duration
type Callable[T any] func() T

// Checker manages the invocation of the callable function, potentially across threads
type Checker[T any] struct {
	accessMutex  sync.RWMutex
	nextCheck    time.Time
	cooldown     time.Duration
	cachedResult T
	fn           Callable[T]
}

// Call executes the callable if it's not in the cooldown period. Otherwise it returns the cached result
func (c *Checker[T]) Call() T {
	result, worked := c.attemptCachedCall()
	if !worked {
		// 1. get write lock
		c.accessMutex.Lock()
		defer c.accessMutex.Unlock()
		// 2. get & set new value
		result = c.fn()
		c.cachedResult = result
		// 3. set next timestamp
		c.nextCheck = time.Now().Add(c.cooldown)
	}
	return result
}

// attemptCachedCall tries to return a cached result if the cooldown period has not yet passed.
// the bool is true if we were successful
func (c *Checker[T]) attemptCachedCall() (T, bool) {
	c.accessMutex.RLock()
	defer c.accessMutex.RUnlock()
	if time.Now().Before(c.nextCheck) {
		return c.cachedResult, true
	}
	var zero T // Return the zero value of T if not successful
	return zero, false
}

// New creates a new Checker with the specified callable and cooldown duration.
func New[T any](c Callable[T], cooldown time.Duration) *Checker[T] {
	checker := Checker[T]{
		cooldown:  cooldown,
		fn:        c,
		nextCheck: time.Now().Add(-2 * cooldown),
	}
	return &checker
}
