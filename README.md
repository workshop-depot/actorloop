# actorloop
a simple implementation of actors in Go

# sample actor
Anything that implements the `Actor` interface can be used as an actor.

```go
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
	for {
		select {
		case <-ctx.Done():
			return nil
		case n := <-sa.numbers:
			if n < 0 {
				return ErrExit
			}
			atomic.AddInt64(&metric, n)
		}
	}
}
```

This actor just increases the metrics by the number it receives. If the context gets canceled or it receives a negative number, it stops.

It can be run like:

```go
NewStarter().
    Start(actor, 1)
```

Which runs the actor just one time. `Starter` will take care of restarting the actor if there are any errors (or panics), the number of times that's specified. Or can restart it for ever if a negative value like `-1` is provided.
