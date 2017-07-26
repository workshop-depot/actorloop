package actorloop

import (
	"context"
	"sync"
	"time"

	"gopkg.in/dc0d/goroutines.v1"
)

//-----------------------------------------------------------------------------
// errors

type serr string

func (v serr) Error() string { return string(v) }

// errors
var (
	ErrExit error = serr("EXIT")
)

//-----------------------------------------------------------------------------
// logger

// Logger simple interface
type Logger interface {
	Printf(format string, v ...interface{})
}

// LoggerFunc helper type providing a simple implementation for Logger
type LoggerFunc func(format string, v ...interface{})

// Printf implements Logger
func (l LoggerFunc) Printf(format string, v ...interface{}) {
	l(format, v...)
}

//-----------------------------------------------------------------------------
// actor

// Actor processes the state in two phase normally:
// listening for events and then processing them.
type Actor interface {
	// Loop is usually a for {} loop (goes forever unless broke out),
	// handling channels and doing stuff. But it can be just a select statement,
	// performing a one time job. Other methods of actor should pass messages to
	// channels that got handled here instead of mutating actor's state directly.
	// A Mailbox is a channel. The context is null if not provided in the Looper.
	Loop(context.Context) error
}

// ActorFunc is a helper implementation of Actor interface for converting
// a single function to an actor.
type ActorFunc func(context.Context) error

// Loop implements Actor interface.
func (af ActorFunc) Loop(ctx context.Context) error {
	return af(ctx)
}

//-----------------------------------------------------------------------------
// looper

// Looper starts an Actor, do not reuse
type Looper struct {
	gutil goroutines.Go
	ctx   context.Context
	l     Logger
}

// New creates a new Looper
func New() Looper { return Looper{} }

// AddToGroup adds the actor's goroutine to the provided *sync.WaitGroup
func (st Looper) AddToGroup(wg *sync.WaitGroup) Looper {
	st.gutil = st.gutil.AddToGroup(wg)
	return st
}

// EnsureStarted ensures that the actor's goroutine got started and then returns
func (st Looper) EnsureStarted() Looper {
	st.gutil = st.gutil.EnsureStarted()
	return st
}

// Before will be called before the goroutine func at the begining of the same goroutine
func (st Looper) Before(before func()) Looper {
	st.gutil = st.gutil.Before(before)
	return st
}

// After will get called after the goroutine func, it can be deferred
func (st Looper) After(after func(), deferred ...bool) Looper {
	st.gutil = st.gutil.After(after, deferred...)
	return st
}

// WithContext passes this context to the actor's Loop method
func (st Looper) WithContext(ctx context.Context) Looper {
	st.ctx = ctx
	return st
}

// WithLogger uses this logger to log errors
func (st Looper) WithLogger(l Logger) Looper {
	st.l = l
	return st
}

// Start starts the actor, default value of period is one second. It restarts the actor
// # intensity number of times. If intensity is negative (-1), the actor would get
// restarted forever.
func (st Looper) Start(
	actor Actor,
	intensity int,
	period ...time.Duration) {
	if st.ctx != nil {
		select {
		case <-st.ctx.Done():
			return
		default:
		}
	}
	if intensity == 0 {
		return
	}
	if intensity > 0 {
		intensity--
	}
	dt := time.Second
	if len(period) > 0 && period[0] > 0 {
		dt = period[0]
	}
	retry := func(e interface{}) {
		if st.l != nil {
			st.l.Printf("error: %v", e)
		}
		time.Sleep(dt)
		go st.Start(actor, intensity, dt)
	}
	st.gutil.
		Recover(func(e interface{}) {
			retry(e)
		}).
		Go(func() {
			switch err := actor.Loop(st.ctx); err {
			case nil, ErrExit:
				return
			default:
				retry(err)
				return
			}
		})
}

//-----------------------------------------------------------------------------
