package eventbus_test

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/trofkm/compoapp/eventbus"
)

// --- Event types ---

type Event1 struct{ ID string }
type Event2 struct{ ID string }
type Event3 struct{ ID string }
type Event4 struct{ ID string }
type Event5 struct{ ID string }
type Event6 struct{ ID string }
type Event7 struct{ ID string }
type Event8 struct{ ID string }
type Event9 struct{ ID string }
type Event10 struct{ ID string }

// --- Helpers ---

func startBus(b *testing.B, bus *eventbus.EventBus) context.CancelFunc {
	b.Helper()
	locked := bus.Build()
	ctx, cancel := context.WithCancel(context.Background())
	ready := make(chan struct{})
	go func() {
		close(ready)
		locked.Start(ctx)
	}()
	<-ready
	runtime.Gosched()
	return cancel
}

// --- 1. Throughput ---

func BenchmarkThroughput_1sub(b *testing.B) {
	bus := eventbus.NewEventBus()
	eventbus.Subscribe(bus, func(_ context.Context, _ Event1) {})
	cancel := startBus(b, bus)
	defer cancel()

	for b.Loop() {
		bus.Publish(Event1{ID: "x"})
	}
}

func BenchmarkThroughput_10sub(b *testing.B) {
	bus := eventbus.NewEventBus()
	for range 10 {
		eventbus.Subscribe(bus, func(_ context.Context, _ Event1) {})
	}
	cancel := startBus(b, bus)
	defer cancel()

	for b.Loop() {
		bus.Publish(Event1{ID: "x"})
	}
}

func BenchmarkThroughput_100sub(b *testing.B) {
	bus := eventbus.NewEventBus()
	for range 100 {
		eventbus.Subscribe(bus, func(_ context.Context, _ Event1) {})
	}
	cancel := startBus(b, bus)
	defer cancel()

	for b.Loop() {
		bus.Publish(Event1{ID: "x"})
	}
}

// --- 2. Latency ---

func BenchmarkLatency(b *testing.B) {
	bus := eventbus.NewEventBus()

	var received atomic.Int64
	eventbus.Subscribe(bus, func(_ context.Context, _ Event1) {
		received.Add(1)
	})
	cancel := startBus(b, bus)
	defer cancel()

	// sanity check
	before := received.Load()
	bus.Publish(Event1{ID: "test"})
	time.Sleep(100 * time.Millisecond)
	after := received.Load()
	// b.Logf("sanity check: before=%d after=%d", before, after)
	if after == before {
		b.Fatal("subscriber never called! bus is broken")
	}

	for b.Loop() {
		before := received.Load()
		bus.Publish(Event1{ID: "x"})
		deadline := time.Now().Add(time.Millisecond)
		for received.Load() == before {
			if time.Now().After(deadline) {
				b.Fatalf("timeout! received=%d", received.Load())
			}
			runtime.Gosched()
		}
	}
}

// --- 3. Multiple event types ---

func BenchmarkMultipleTypes_10(b *testing.B) {
	bus := eventbus.NewEventBus()
	eventbus.Subscribe(bus, func(_ context.Context, _ Event1) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event2) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event3) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event4) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event5) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event6) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event7) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event8) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event9) {})
	eventbus.Subscribe(bus, func(_ context.Context, _ Event10) {})
	cancel := startBus(b, bus)
	defer cancel()

	events := []any{
		Event1{}, Event2{}, Event3{}, Event4{}, Event5{},
		Event6{}, Event7{}, Event8{}, Event9{}, Event10{},
	}

	for i := range b.N {
		bus.Publish(events[i%10])
	}
}

// --- 4. Concurrent publishers ---

func BenchmarkConcurrentPublish_2(b *testing.B)  { benchConcurrent(b, 2) }
func BenchmarkConcurrentPublish_8(b *testing.B)  { benchConcurrent(b, 8) }
func BenchmarkConcurrentPublish_32(b *testing.B) { benchConcurrent(b, 32) }

func benchConcurrent(b *testing.B, goroutines int) {
	b.Helper()
	bus := eventbus.NewEventBus()

	var count atomic.Int64
	eventbus.Subscribe(bus, func(_ context.Context, _ Event1) {
		count.Add(1)
	})
	cancel := startBus(b, bus)
	defer cancel()

	b.SetParallelism(goroutines)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bus.Publish(Event1{})
		}
	})

	// дренируем остатки
	expected := int64(b.N)
	deadline := time.Now().Add(5 * time.Second)
	for count.Load() < expected {
		if time.Now().After(deadline) {
			break
		}
		runtime.Gosched()
	}
}

// --- 5. No subscribers ---

func BenchmarkPublishNoSubs(b *testing.B) {
	bus := eventbus.NewEventBus()
	cancel := startBus(b, bus)
	defer cancel()

	for b.Loop() {
		bus.Publish(Event1{})
	}
}
