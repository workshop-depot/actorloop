package actorloop

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/dc0d/goroutines.v1"
)

var (
	ctx    context.Context
	cancel context.CancelFunc
	wg     = &sync.WaitGroup{}
)

func TestMain(m *testing.M) {
	defer func() {
		goroutines.New().
			Timeout(time.Second).
			Go(func() {
				wg.Wait()
			})
	}()
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()

	m.Run()
}

var metric int64

type sampleActor struct {
	numbers chan int64
}

func newSampleActor() *sampleActor {
	res := sampleActor{
		numbers: make(chan int64),
	}
	return &res
}

func (sa *sampleActor) add(n int64) {
	sa.numbers <- n
}

func (sa *sampleActor) Loop(ctx context.Context) error {
	done := func() <-chan struct{} {
		if ctx != nil {
			return ctx.Done()
		}
		return nil
	}
	for {
		select {
		case <-done():
			return nil
		case n := <-sa.numbers:
			if n < 0 {
				return ErrExit
			}
			atomic.AddInt64(&metric, n)
		}
	}
}

func TestSimpleForm(t *testing.T) {
	metric = 0
	actor := newSampleActor()

	NewStarter().
		Start(actor, 1)

	lwg := &sync.WaitGroup{}
	lwg.Add(1)
	go func() {
		defer lwg.Done()
		actor.add(30)
		actor.add(30)
		actor.add(30)
		actor.add(-1)
	}()
	lwg.Wait()

	assert.Equal(t, int64(90), atomic.LoadInt64(&metric))
}

func TestMessaging(t *testing.T) {
	metric = 0
	actor := newSampleActor()

	NewStarter().
		AddToGroup(wg).
		WithContext(ctx).
		EnsureStarted().
		WithLogger(LoggerFunc(log.Printf)).
		Start(actor, 1)

	go func() {
		actor.add(30)
		actor.add(30)
		actor.add(30)
		actor.add(-1)
	}()

	wg.Wait()
	assert.Equal(t, int64(90), atomic.LoadInt64(&metric))
}

func TestContext(t *testing.T) {
	metric = 0
	lctx, lcancel := context.WithCancel(ctx)
	lcancel()
	lwg := &sync.WaitGroup{}

	actor := newSampleActor()
	NewStarter().
		AddToGroup(lwg).
		WithContext(lctx).
		EnsureStarted().
		WithLogger(LoggerFunc(log.Printf)).
		Start(actor, 1)

	lwg.Wait()
	assert.Equal(t, int64(0), atomic.LoadInt64(&metric))
}

func TestActorFunc(t *testing.T) {
	metric = 0

	numbers := make(chan int64)
	var af ActorFunc = func(c context.Context) error {
		select {
		case <-c.Done():
			return nil
		case n := <-numbers:
			atomic.AddInt64(&metric, n)
			return ErrExit
		}
	}

	NewStarter().
		AddToGroup(wg).
		WithContext(ctx).
		EnsureStarted().
		WithLogger(LoggerFunc(log.Printf)).
		Start(af, 1)

	go func() {
		numbers <- 30
	}()

	wg.Wait()
	assert.Equal(t, int64(30), atomic.LoadInt64(&metric))
}

func TestError(t *testing.T) {
	metric = 0
	lwg := &sync.WaitGroup{}

	numbers := make(chan int64)
	var af ActorFunc = func(c context.Context) error {
		select {
		case <-c.Done():
			return nil
		case n := <-numbers:
			atomic.AddInt64(&metric, n)
			return serr("SOME OTHER ERROR")
		}
	}

	NewStarter().
		AddToGroup(lwg).
		WithContext(ctx).
		EnsureStarted().
		Start(af, 3, time.Millisecond*10)

	lwg.Add(1)
	go func() {
		defer lwg.Done()
		numbers <- 30
		numbers <- 30
		numbers <- 30
	}()

	lwg.Wait()
	assert.Equal(t, int64(90), atomic.LoadInt64(&metric))
}

func TestPanic(t *testing.T) {
	metric = 0
	lwg := &sync.WaitGroup{}

	numbers := make(chan int64)
	var af ActorFunc = func(c context.Context) error {
		select {
		case <-c.Done():
			return nil
		case n := <-numbers:
			atomic.AddInt64(&metric, n)
			panic("SOME OTHER ERROR")
		}
	}

	NewStarter().
		AddToGroup(lwg).
		WithContext(ctx).
		EnsureStarted().
		Start(af, 3, time.Millisecond*10)

	lwg.Add(1)
	go func() {
		defer lwg.Done()
		numbers <- 30
		numbers <- 30
		numbers <- 30
	}()

	lwg.Wait()
	assert.Equal(t, int64(90), atomic.LoadInt64(&metric))
}
