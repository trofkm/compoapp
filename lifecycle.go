package compoapp

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Initer is a component which has some initialization logic before Start
type Initer interface {
	Init(ctx context.Context) error
}

// Starter is component which has Start method.
type Starter interface {
	Start(ctx context.Context) error
}

// Redier is a component that has some long Start logic, so he must explicitly say when he is ready
type Redier interface {
	// Ready returns channel which must be closed when component is ready
	Ready() <-chan struct{}
}

// LifecycleRunner encapsulates the init and start logic.
//
// It launches the Init() and Start() with correct order and automatically waits for component to be started
type LifecycleRunner struct {
	container *Container
	target    any
	// responsible for logs
	debug bool
}

// ResolveLifecycle creates LifecycleRunner from container
func (c *Container) ResolveLifecycle(target any) *LifecycleRunner {
	return &LifecycleRunner{container: c, target: target, debug: c.debug}
}

func (r *LifecycleRunner) Execute(ctx context.Context) error {
	if err := r.container.Resolve(r.target); err != nil {
		return fmt.Errorf("resolve: %w", err)
	}

	for _, component := range r.container.sorted {
		if i, ok := component.(Initer); ok {
			r.debugf("calling %T.Init(ctx)", i)
			if err := i.Init(ctx); err != nil {
				return fmt.Errorf("init %T: %w", component, err)
			}
		}
	}

	// component -> ready channels of its dependencies
	readiers := make(map[any][]<-chan struct{})

	for typ, val := range r.container.instances {
		componentVal := r.container.instances[typ]
		depTypes, ok := r.container.graph.dependencies[typ]
		if !ok {
			continue
		}

		r.debugf("collecting ready statuses for %T", val)
		for _, depType := range depTypes {
			depVal, ok := r.container.instances[depType]
			if !ok {
				continue
			}

			if readier, ok := depVal.(Redier); ok {
				r.debugf("found %T with Ready() method", depVal)
				readiers[componentVal] = append(readiers[componentVal], readier.Ready())
			}
		}
	}
	// todo: can be implement in more convenient way?
	eg, ctx := errgroup.WithContext(ctx)

	for _, component := range r.container.sorted {
		s, ok := component.(Starter)
		if !ok {
			continue
		}

		eg.Go(func() error {
			// waiting for dependency resolution (Start method called)
			if depsChans, ok := readiers[component]; ok {
				wg := sync.WaitGroup{}
				for _, ch := range depsChans {
					wg.Go(func() {
						<-ch
					})
				}
				wg.Wait()
			}

			if err := s.Start(ctx); err != nil {
				return fmt.Errorf("start %T: %w", component, err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}

func (r *LifecycleRunner) debugf(format string, args ...any) {
	if r.debug {
		fmtStr := "[LIFECYCLE] " + format + "\n"
		fmt.Printf(fmtStr, args...)
	}
}
