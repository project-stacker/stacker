package overlay

import (
	"context"
	"sync"

	"github.com/pkg/errors"
)

var ThreadPoolCancelled = errors.Errorf("thread pool cancelled")

type ThreadPool struct {
	ctx    context.Context
	cancel context.CancelFunc
	n      int
	tasks  chan func(context.Context) error
	err    error
}

func NewThreadPool(n int) *ThreadPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &ThreadPool{ctx, cancel, n, make(chan func(context.Context) error, 1000), nil}
}

func (tp *ThreadPool) Add(f func(context.Context) error) {
	tp.tasks <- f
}

func (tp *ThreadPool) DoneAddingJobs() {
	close(tp.tasks)
}

func (tp *ThreadPool) Run() error {
	wg := sync.WaitGroup{}
	wg.Add(tp.n)
	for i := 0; i < tp.n; i++ {
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-tp.ctx.Done():
					return
				case f, ok := <-tp.tasks:
					if !ok {
						return
					}

					err := f(tp.ctx)
					if err != nil && err != ThreadPoolCancelled {
						tp.err = err
						tp.cancel()
						return
					}
				}
			}
		}(i)
	}

	wg.Wait()
	return tp.err
}
