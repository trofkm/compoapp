package eventbus

import (
	"context"
	"reflect"
	"sync"
)

type subscriber func(ctx context.Context, data any)

type Subscriber[T any] func(ctx context.Context, data T)

type EventBus struct {
	// event and channel associated with event
	events map[reflect.Type][]chan any
	// all subscribers for event
	subs map[reflect.Type][]subscriber
}

func NewEventBus() *EventBus {
	return &EventBus{
		events: make(map[reflect.Type][]chan any),
		subs:   make(map[reflect.Type][]subscriber),
	}
}

func (e *EventBus) Publish(data any) {
	typ := reflect.TypeOf(data)
	chans, ok := e.events[typ]
	if !ok {
		return
	}

	// fan out
	for _, ch := range chans {
		ch <- data
	}
}

func Subscribe[T any](bus *EventBus, sub Subscriber[T]) {
	var zero T
	typ := reflect.TypeOf(&zero).Elem()
	ch := make(chan any, 128)
	bus.events[typ] = append(bus.events[typ], ch)
	bus.subs[typ] = append(bus.subs[typ], func(ctx context.Context, data any) {
		sub(ctx, data.(T))
	})
}

// Build is necessary because we shouldn't change subscribers after Start
func (e *EventBus) Build() *LockedEventBus {
	return &LockedEventBus{e}
}

type LockedEventBus struct {
	bus *EventBus
}

func (l *LockedEventBus) Start(ctx context.Context) error {
	e := l.bus
	wg := sync.WaitGroup{}

	for ev, chans := range e.events {
		for i, ch := range chans {
			sub := e.subs[ev][i]
			wg.Go(func() {
				for {
					select {
					case <-ctx.Done():
						return

					case data := <-ch:
						sub(ctx, data)
					}
				}
			})
		}
	}

	wg.Wait()

	return nil
}
