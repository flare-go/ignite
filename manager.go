package ignite

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"go.uber.org/atomic"
)

// Manager interface defines the operations for managing multiple object pools
type Manager interface {
	CloseAll()
	RegisterPool(t reflect.Type, config Config[any]) error
	GetPool(t reflect.Type) (Pool[any], error)
	UnregisterPool(t reflect.Type) error
	ListPoolTypes() []reflect.Type
	GetOrCreatePool(t reflect.Type, config Config[any]) (Pool[any], error)
	Stats() map[reflect.Type]Stats
	ResizePool(t reflect.Type, newSize int) error
}

// manager struct implements the Manager interface
type manager struct {
	pools     sync.Map
	mu        sync.RWMutex
	closed    atomic.Bool
	ctx       context.Context
	cancelCtx context.CancelFunc
}

// NewManager creates and returns a new Manager instance
func NewManager() Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &manager{
		pools:     sync.Map{},
		ctx:       ctx,
		cancelCtx: cancel,
	}
}

// CloseAll closes all managed pools
func (m *manager) CloseAll() {
	if m.closed.CompareAndSwap(false, true) {
		m.cancelCtx()
		m.pools.Range(func(key, value any) bool {
			if p, ok := value.(Pool[any]); ok {
				if err := p.Close(context.Background()); err != nil {
					return false
				}
			}
			return true
		})
	}
}

// RegisterPool registers a new pool for a given type
func (m *manager) RegisterPool(t reflect.Type, config Config[any]) error {
	if m.closed.Load() {
		return ErrPoolClosed
	}

	_, loaded := m.pools.LoadOrStore(t, atomic.Value{})
	if loaded {
		return fmt.Errorf("pool already registered for type: %v", t)
	}

	pool, err := NewPool(config)
	if err != nil {
		m.pools.Delete(t)
		return fmt.Errorf("failed to create pool: %w", err)
	}

	value := atomic.Value{}
	value.Store(pool)
	m.pools.Store(t, &value)
	return nil
}

// GetPool retrieves a pool for a given type
func (m *manager) GetPool(t reflect.Type) (Pool[any], error) {
	if m.closed.Load() {
		return nil, ErrPoolClosed
	}

	if poolValue, ok := m.pools.Load(t); ok {
		if atomicValue, ok := poolValue.(*atomic.Value); ok {
			return atomicValue.Load().(Pool[any]), nil
		}
	}
	return nil, fmt.Errorf("pool not found for type: %v", t)
}

// UnregisterPool removes a pool for a given type
func (m *manager) UnregisterPool(t reflect.Type) error {
	if m.closed.Load() {
		return ErrPoolClosed
	}

	if poolValue, ok := m.pools.LoadAndDelete(t); ok {
		if atomicValue, ok := poolValue.(*atomic.Value); ok {
			if pool, ok := atomicValue.Load().(Pool[any]); ok {
				if err := pool.Close(context.Background()); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return fmt.Errorf("pool not found for type: %v", t)
}

// ListPoolTypes returns a list of all registered pool types
func (m *manager) ListPoolTypes() []reflect.Type {
	var types []reflect.Type
	m.pools.Range(func(key, value any) bool {
		if t, ok := key.(reflect.Type); ok {
			types = append(types, t)
		}
		return true
	})
	return types
}

// GetOrCreatePool retrieves an existing pool or creates a new one if it doesn't exist
func (m *manager) GetOrCreatePool(t reflect.Type, config Config[any]) (Pool[any], error) {
	if m.closed.Load() {
		return nil, ErrPoolClosed
	}

	poolValue, loaded := m.pools.LoadOrStore(t, &atomic.Value{})
	if loaded {
		if atomicValue, ok := poolValue.(*atomic.Value); ok {
			return atomicValue.Load().(Pool[any]), nil
		}
	}

	pool, err := NewPool(config)
	if err != nil {
		m.pools.Delete(t)
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	atomicValue := poolValue.(*atomic.Value)
	atomicValue.Store(pool)
	return pool, nil
}

// Stats return the statistics for all managed pools
func (m *manager) Stats() map[reflect.Type]Stats {
	stats := make(map[reflect.Type]Stats)
	m.pools.Range(func(key, value any) bool {
		if t, ok := key.(reflect.Type); ok {
			if atomicValue, ok := value.(*atomic.Value); ok {
				if p, ok := atomicValue.Load().(Pool[any]); ok {
					stats[t] = p.Stats()
				}
			}
		}
		return true
	})
	return stats
}

// ResizePool changes the size of a specific pool
func (m *manager) ResizePool(t reflect.Type, newSize int) error {
	pool, err := m.GetPool(t)
	if err != nil {
		return err
	}
	return pool.Resize(newSize)
}
