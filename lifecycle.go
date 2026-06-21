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
	mu          sync.RWMutex
	readiers    map[any][]<-chan struct{}
	initialized map[any]struct{}
	container   *Container
	targets     []any
	// responsible for logs
	debug bool
}

// ResolveLifecycle creates LifecycleRunner from container for multiple top-level targets.
func (c *Container) ResolveLifecycle(targets ...any) *LifecycleRunner {
	// todo: add debug mode?
	return &LifecycleRunner{
		readiers:    make(map[any][]<-chan struct{}),
		initialized: make(map[any]struct{}),
		container:   c,
		targets:     targets,
		debug:       c.debug,
	}
}

func (r *LifecycleRunner) Execute(ctx context.Context) error {
	eg, ctx := errgroup.WithContext(ctx)

	for _, t := range r.targets {
		eg.Go(func() error {
			return r.executeOne(ctx, t)
		})
	}
	return eg.Wait()
}

func (r *LifecycleRunner) executeOne(ctx context.Context, target any) error {
	if err := r.container.Resolve(target); err != nil {
		return fmt.Errorf("resolve: %w", err)
	}

	for _, component := range r.container.sorted {
		if i, ok := component.(Initer); ok {
			if _, ok := r.initialized[component]; ok {
				// component already initialized
				continue
			}
			r.initialized[component] = struct{}{}
			r.debugf("calling %T.Init(ctx)", i)
			// here we have to collect initialized components
			if err := i.Init(ctx); err != nil {
				return fmt.Errorf("init %T: %w", component, err)
			}

		}
	}

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
