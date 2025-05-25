package bee

import (
	"context"
	"sync"
	"sync/atomic"
)

func New(ctx context.Context, size int) *Pool {
	done := make(chan struct{})
	closeDone := sync.OnceFunc(func() { close(done) })
	return &Pool{
		ctx:  ctx,
		work: make(chan struct{}, size),
		done: done, closeDone: closeDone,
	}
}

type Pool struct {
	ctx       context.Context
	work      chan struct{}
	done      chan struct{}
	closeDone context.CancelFunc
	running   atomic.Int32
	worked    atomic.Int64
	index     atomic.Int64
}

func (p *Pool) Run(f func()) bool {
	return p.RunWithContextAndIndex(func(context.Context, int64) { f() })
}

func (p *Pool) RunWithContext(f func(context.Context)) bool {
	return p.RunWithContextAndIndex(func(ctx context.Context, _ int64) { f(ctx) })
}

func (p *Pool) RunWithContextAndIndex(f func(ctx context.Context, index int64)) bool {
	select {
	case <-p.ctx.Done():
		p.closeDone()
		return false
	case <-p.done:
		return false
	case p.work <- struct{}{}:
		p.running.Add(1)
		go func() {
			defer func() {
				<-p.work
				p.running.Add(-1)
				p.worked.Add(1)
			}()
			defer recover()
			f(p.ctx, p.index.Add(1))
		}()
		return true
	}
}

func (p *Pool) Worked() int64 {
	return p.worked.Load()
}

func (p *Pool) Running() int32 {
	return p.running.Load()
}
