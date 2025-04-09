/*	A small package to help manage resources that should be unique per-thread
 */
package threadboundresourcepool

import (
	"sync"
)

type FactoryFn[T any] func() T

type ThreadResource[T any] struct {
	resources map[int]T    // one connection per goroutine
	mutex     sync.RWMutex // protexts the dbConnections map (only concurrent reads are ok)
	factory   FactoryFn[T]
}

func New[T any](factory FactoryFn[T]) *ThreadResource[T] {
	t := ThreadResource[T]{
		resources: make(map[int]T),
		factory:   factory,
	}
	return &t
}

func (tr *ThreadResource[T]) GetResource(threadID int) T {
	tr.mutex.RLock()
	res, ok := tr.resources[threadID]
	tr.mutex.RUnlock()
	if !ok {
		res = tr.factory()
		tr.mutex.Lock()
		defer tr.mutex.Unlock()
		tr.resources[threadID] = res
	}
	return res
}
