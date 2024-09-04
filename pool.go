package ignite

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/atomic"
)

var (
	ErrPoolExhausted = errors.New("pool is exhausted")
	ErrPoolClosed    = errors.New("pool is closed")
	ErrInvalidConfig = errors.New("invalid pool configuration")
)

type Pool[T any] interface {
	Get(ctx context.Context) (*ObjectWrapper[T], error)
	Put(*ObjectWrapper[T])
	Close(ctx context.Context) error
	Len() int64
	Stats() Stats
	Resize(newSize int) error
	UpdateConfig(newConfig Config[T]) error
}

type ObjectWrapper[T any] struct {
	Object      T
	CreateTime  time.Time
	LastUseTime atomic.Time
	UsageCount  atomic.Int64
}

type Config[T any] struct {
	InitialSize         int
	MaxSize             int
	MinSize             int
	MaxIdleTime         time.Duration
	Factory             func() (T, error)
	Reset               func(T) error
	Validate            func(T) error
	HealthCheck         func(T) error
	HealthCheckInterval time.Duration
}

type Stats struct {
	CurrentSize      atomic.Int64
	AvailableObjects atomic.Int64
	InUseObjects     atomic.Int64
	TotalCreated     atomic.Int64
	TotalDestroyed   atomic.Int64
	TotalResets      atomic.Int64
	MaxUsage         atomic.Int64
}

type pool[T any] struct {
	config         Config[T]
	objects        chan *ObjectWrapper[T]
	closed         atomic.Bool
	stats          Stats
	mu             sync.RWMutex
	configUpdateCh chan Config[T]
	stopCh         chan struct{}
	wrapperPool    sync.Pool
}

func NewPool[T any](config Config[T]) (Pool[T], error) {
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	p := &pool[T]{
		config:         config,
		objects:        make(chan *ObjectWrapper[T], config.MaxSize),
		configUpdateCh: make(chan Config[T]),
		stopCh:         make(chan struct{}),
		wrapperPool: sync.Pool{
			New: func() interface{} {
				return &ObjectWrapper[T]{}
			},
		},
	}

	for i := 0; i < config.InitialSize; i++ {
		if err := p.addObject(); err != nil {
			if err = p.Close(context.Background()); err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("failed to create initial objects: %w", err)
		}
	}

	go p.mainLoop()

	return p, nil
}

func (p *pool[T]) mainLoop() {
	cleanupTicker := time.NewTicker(p.config.MaxIdleTime / 2)
	defer cleanupTicker.Stop()

	var healthCheckTicker *time.Ticker
	if p.config.HealthCheck != nil && p.config.HealthCheckInterval > 0 {
		healthCheckTicker = time.NewTicker(p.config.HealthCheckInterval)
		defer healthCheckTicker.Stop()
	}

	for {
		select {
		case <-p.stopCh:
			return
		case newConfig := <-p.configUpdateCh:
			p.updateConfig(newConfig)
			cleanupTicker.Reset(newConfig.MaxIdleTime / 2)
			if healthCheckTicker != nil {
				healthCheckTicker.Stop()
			}
			if newConfig.HealthCheck != nil && newConfig.HealthCheckInterval > 0 {
				healthCheckTicker = time.NewTicker(newConfig.HealthCheckInterval)
			}
		case <-cleanupTicker.C:
			p.cleanup()
		case <-healthCheckTicker.C:
			p.performHealthCheck()
		}
	}
}

func (p *pool[T]) updateConfig(newConfig Config[T]) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.config = newConfig

	if newConfig.MaxSize < len(p.objects) {
		excess := len(p.objects) - newConfig.MaxSize
		for i := 0; i < excess; i++ {
			select {
			case obj := <-p.objects:
				p.destroyObject(obj)
			default:
				return
			}
		}
	}
}

func (p *pool[T]) Get(ctx context.Context) (*ObjectWrapper[T], error) {
	if p.closed.Load() {
		return nil, ErrPoolClosed
	}

	for {
		select {
		case obj := <-p.objects:
			p.stats.AvailableObjects.Dec()
			if err := p.prepareObject(obj); err != nil {
				// 如果物件準備失敗，我們繼續嘗試獲取下一個物件
				continue
			}
			return obj, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			// 如果通道為空，我們嘗試添加一個新物件
			if p.stats.CurrentSize.Load() < int64(p.config.MaxSize) {
				if err := p.addObject(); err != nil {
					return nil, fmt.Errorf("failed to create new object: %w", err)
				}
				// 添加成功後，繼續循環嘗試獲取物件
				continue
			}
			// 如果池已滿，返回錯誤
			return nil, ErrPoolExhausted
		}
	}
}

func (p *pool[T]) prepareObject(obj *ObjectWrapper[T]) error {
	if time.Since(obj.LastUseTime.Load()) > p.config.MaxIdleTime {
		if err := p.resetObject(obj); err != nil {
			p.destroyObject(obj)
			return fmt.Errorf("failed to reset object: %w", err)
		}
	}

	if p.config.Validate != nil {
		if err := p.config.Validate(obj.Object); err != nil {
			p.destroyObject(obj)
			return fmt.Errorf("object validation failed: %w", err)
		}
	}

	obj.LastUseTime.Store(time.Now())
	obj.UsageCount.Inc()
	p.stats.InUseObjects.Inc()
	p.updateMaxUsage(obj.UsageCount.Load())
	return nil
}

func (p *pool[T]) Put(obj *ObjectWrapper[T]) {
	if p.closed.Load() {
		p.destroyObject(obj)
		return
	}

	obj.LastUseTime.Store(time.Now())
	p.stats.InUseObjects.Dec()

	if time.Since(obj.LastUseTime.Load()) > p.config.MaxIdleTime {
		p.destroyObject(obj)
		return
	}

	select {
	case p.objects <- obj:
		p.stats.AvailableObjects.Inc()
	default:
		p.destroyObject(obj)
	}
}

func (p *pool[T]) Close(ctx context.Context) error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(p.stopCh)

	// 使用帶超時的上下文進行關閉操作
	closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		close(p.objects)
		for obj := range p.objects {
			p.destroyObject(obj)
		}
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-closeCtx.Done():
		return closeCtx.Err()
	}
}

func (p *pool[T]) Len() int64 {
	return p.stats.CurrentSize.Load()
}

func (p *pool[T]) Stats() Stats {
	return Stats{
		CurrentSize:      p.stats.CurrentSize,
		AvailableObjects: atomic.Int64{},
		InUseObjects:     p.stats.InUseObjects,
		TotalCreated:     p.stats.TotalCreated,
		TotalDestroyed:   p.stats.TotalDestroyed,
		TotalResets:      p.stats.TotalResets,
		MaxUsage:         p.stats.MaxUsage,
	}
}

func (p *pool[T]) Resize(newSize int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if newSize < p.config.MinSize || newSize > p.config.MaxSize {
		return fmt.Errorf("new size must be between %d and %d", p.config.MinSize, p.config.MaxSize)
	}

	currentSize := p.stats.CurrentSize.Load()
	if newSize > int(currentSize) {
		for i := currentSize; i < int64(newSize); i++ {
			if err := p.addObject(); err != nil {
				return fmt.Errorf("failed to add object during resize: %w", err)
			}
		}
	} else if newSize < int(currentSize) {
		for i := currentSize; i > int64(newSize); i-- {
			select {
			case obj := <-p.objects:
				p.destroyObject(obj)
			default:
				return nil
			}
		}
	}

	return nil
}

func (p *pool[T]) UpdateConfig(newConfig Config[T]) error {
	if err := validateConfig(newConfig); err != nil {
		return err
	}
	p.configUpdateCh <- newConfig
	return nil
}

func (p *pool[T]) addObject() error {
	obj, err := p.config.Factory()
	if err != nil {
		return err
	}

	wrapper := p.wrapperPool.Get().(*ObjectWrapper[T])
	wrapper.Object = obj
	wrapper.CreateTime = time.Now()
	wrapper.LastUseTime.Store(time.Now())
	wrapper.UsageCount.Store(0)

	select {
	case p.objects <- wrapper:
		p.stats.CurrentSize.Inc()
		p.stats.TotalCreated.Inc()
		p.stats.AvailableObjects.Inc()
		return nil
	default:
		p.wrapperPool.Put(wrapper)
		if p.config.Reset != nil {
			_ = p.config.Reset(obj)
		}
		return errors.New("failed to add object to pool: channel full")
	}
}

func (p *pool[T]) cleanup() {
	var activeObjs []*ObjectWrapper[T]
	batchSize := 100 // 可以根據實際情況調整

	for i := 0; i < batchSize; i++ {
		select {
		case obj := <-p.objects:
			if time.Since(obj.LastUseTime.Load()) > p.config.MaxIdleTime && p.stats.CurrentSize.Load() > int64(p.config.MinSize) {
				p.destroyObject(obj)
			} else {
				activeObjs = append(activeObjs, obj)
			}
		default:
			break
		}
	}

	for _, obj := range activeObjs {
		p.objects <- obj
	}
}

func (p *pool[T]) performHealthCheck() {
	var healthyObjs []*ObjectWrapper[T]
	batchSize := 100 // 可以根據實際情況調整

	for i := 0; i < batchSize; i++ {
		select {
		case obj := <-p.objects:
			if err := p.config.HealthCheck(obj.Object); err != nil {
				p.destroyObject(obj)
			} else {
				healthyObjs = append(healthyObjs, obj)
			}
		default:
			break
		}
	}

	for _, obj := range healthyObjs {
		p.objects <- obj
	}
}

func validateConfig[T any](config Config[T]) error {
	if config.InitialSize < 0 || config.MaxSize <= 0 || config.InitialSize > config.MaxSize || config.MinSize > config.MaxSize {
		return ErrInvalidConfig
	}
	if config.MaxIdleTime <= 0 {
		return errors.New("MaxIdleTime must be positive")
	}
	if config.Factory == nil {
		return errors.New("factory function must be provided")
	}
	if config.HealthCheck != nil && config.HealthCheckInterval <= 0 {
		return errors.New("HealthCheckInterval must be positive when HealthCheck is provided")
	}
	return nil
}

func (p *pool[T]) destroyObject(obj *ObjectWrapper[T]) {
	if p.config.Reset != nil {
		_ = p.config.Reset(obj.Object)
	}
	p.wrapperPool.Put(obj)
	p.stats.CurrentSize.Dec()
	p.stats.TotalDestroyed.Inc()
	p.stats.AvailableObjects.Dec()
}

func (p *pool[T]) resetObject(obj *ObjectWrapper[T]) error {
	if p.config.Reset != nil {
		if err := p.config.Reset(obj.Object); err != nil {
			return fmt.Errorf("failed to reset object: %w", err)
		}
	}
	p.stats.TotalResets.Inc()
	return nil
}

func (p *pool[T]) updateMaxUsage(currentUsage int64) {
	for {
		maxUsage := p.stats.MaxUsage.Load()
		if currentUsage <= maxUsage {
			return
		}
		if p.stats.MaxUsage.CompareAndSwap(maxUsage, currentUsage) {
			return
		}
	}
}
