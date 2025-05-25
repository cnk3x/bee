package bee

import (
	"context"
	"sync"
	"sync/atomic"
)

// New 创建一个带有缓冲区的Pool，用于管理工作线程。
//
// 参数:
//
//	ctx context.Context: 上下文，用于控制工作线程的生命周期。
//	size int: 缓冲区大小，表示可以同时处理的任务数量。
//
// 返回值:
//
//	*Pool: 指向新创建的Pool实例的指针。
func New(ctx context.Context, size int) *Pool {
	// 创建一个用于通知所有任务完成的通道。
	done := make(chan struct{})
	// 创建一个只执行一次的函数，用于关闭done通道。
	closeDone := sync.OnceFunc(func() { close(done) })
	// 返回一个新的Pool实例。
	return &Pool{
		ctx:       ctx,
		work:      make(chan struct{}, size),
		done:      done,
		closeDone: closeDone,
	}
}

// Pool 定义了一个线程池结构体，用于管理工作线程。
type Pool struct {
	ctx       context.Context    // 上下文，用于控制工作线程的生命周期
	work      chan struct{}      // 用于限制并发任务数的通道
	done      chan struct{}      // 用于通知所有任务完成的通道
	closeDone context.CancelFunc // 用于关闭done通道的函数
	running   atomic.Int32       // 当前正在运行的任务数量
	worked    atomic.Int64       // 已完成的任务数量
	index     atomic.Int64       // 任务的索引计数器
}

// Run 执行一个任务，无需上下文和索引。
//
// 参数:
//
//	f func(): 要执行的任务函数。
//
// 返回值:
//
//	bool: 表示任务是否已提交到线程池中执行。
func (p *Pool) Run(f func()) bool {
	return p.RunWithContextAndIndex(func(context.Context, int64) { f() })
}

// RunWithContext 执行一个需要上下文的任务。
//
// 参数:
//
//	f func(context.Context): 要执行的任务函数，接受上下文作为参数。
//
// 返回值:
//
//	bool: 表示任务是否已提交到线程池中执行。
func (p *Pool) RunWithContext(f func(context.Context)) bool {
	return p.RunWithContextAndIndex(func(ctx context.Context, _ int64) { f(ctx) })
}

// RunWithContextAndIndex 执行一个需要上下文和索引的任务。
//
// 参数:
//
//	f func(ctx context.Context, index int64): 要执行的任务函数，接受上下文和索引作为参数。
//
// 返回值:
//
//	bool: 表示任务是否已提交到线程池中执行。
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

// Exit 启动线程池的退出过程。
func (p *Pool) Exit() {
	p.closeDone()
}

// Worked 返回已完成任务的数量。
//
// 返回值:
//
//	int64: 完成任务的数量。
func (p *Pool) Worked() int64 {
	return p.worked.Load()
}

// Running 返回当前正在运行的任务数量。
//
// 返回值:
//
//	int32: 正在运行的任务数量。
func (p *Pool) Running() int32 {
	return p.running.Load()
}
