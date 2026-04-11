package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/trofkm/compoapp/eventbus"
)

func main() {
	b := eventbus.NewEventBus()

	eventbus.Subscribe(b, func(ctx context.Context, data string) {
		fmt.Println("received string: ", data)
	})

	eventbus.Subscribe(b, func(ctx context.Context, data int) {
		fmt.Println("received number: ", data)
	})

	wg := sync.WaitGroup{}

	wg.Go(func() {
		for i := range 10 {
			b.Publish(i)
			time.Sleep(1 * time.Millisecond)
		}
	})

	wg.Go(func() {
		for i := range 10 {
			b.Publish(strconv.Itoa(i + 1000))
			time.Sleep(1 * time.Millisecond)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()

	errChan := make(chan error)
	go func() {
		defer close(errChan)
		errChan <- b.Build().Start(ctx)
	}()

	wg.Wait()

	<-errChan
}
