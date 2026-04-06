# compoapp

A small dependency injection container for Go. ~600 lines, no external dependencies.

## Install

```bash
go get github.com/trofkm/compoapp
```

## Basic usage

Register constructors, resolve the root type. Dependencies are wired automatically by parameter types.

```go
container := compoapp.NewContainer()
container.MustProvide(NewDatabase)
container.MustProvide(NewUserService)
container.MustProvide(NewHTTPServer)

var server *HTTPServer
container.MustResolve(&server)
```

The container builds a dependency graph, topologically sorts it, and constructs types in the correct order. Circular dependencies are detected and reported as errors.

## Lifecycle

For applications that need controlled startup and shutdown, use `ResolveLifecycle` instead of `MustResolve`.

```go
var server *HTTPServer
if err := container.ResolveLifecycle(&server).Execute(ctx); err != nil {
    log.Fatal(err)
}
```

`Execute` runs three stages in order, then blocks until `ctx` is cancelled:

```
1. construct — all types built in dependency order
2. init      — sequential, blocking, fail-fast
3. start     — launched by lifecycle runner concurrently, each component waits for its dependencies to be ready
```

Each stage is opt-in via interfaces:

```go
type Initer interface {
    Init(ctx context.Context) error
}

type Starter interface {
    Start(ctx context.Context) error
}

type Readier interface {
    Ready() <-chan struct{}
}
```

A component implements only what it needs. `Config` might implement none. `Database` might implement all three.

**Ordering guarantee:** if `HTTPServer` depends on `Database`, then `Database.Init`, `Database.Start`, and `Database.Ready()` all complete before `HTTPServer.Start` is called.

**Graceful shutdown** is the component's own responsibility via `ctx.Done()`:

```go
type Database struct {
	ready chan struct{}
}

func NewDatabase() *Database {
	// make it buffered
	return &Database{ready: make(chan struct{}, 1)}
}

func (d *Database) Start(ctx context.Context) error {
	// startup work...
	d.ready <- struct{}
	close(d.ready)

	<-ctx.Done()
	// shutdown work...
	d.conn.Close()

	return nil
}

func (d *Database) Ready() <-chan struct{} {
	return d.ready // same channel every time, created in constructor
}
```

Full example with a realistic dependency tree: [samples/lifecycle](samples/lifecycle)

## API

```go
func NewContainer() *Container
func (c *Container) Provide(constructor interface{}) error
func (c *Container) MustProvide(constructor interface{})
func (c *Container) Resolve(target interface{}) error
func (c *Container) MustResolve(target interface{})
func (c *Container) Debug()
func (c *Container) Visualize(pathToDot string) error
func (c *Container) ResolveLifecycle(target interface{}) *LifecycleRunner
func (r *LifecycleRunner) Execute(ctx context.Context) error
```

## Roadmap

- [x] Dependency resolution with reflection
- [x] Topological sorting and circular dependency detection
- [x] Thread-safe container operations
- [x] Interface binding support
- [x] Lifecycle support (Init, Start, Ready)
- [ ] Named/tagged dependencies
- [ ] Scope support
- [ ] Init/Start timeout

## Limitations

- Constructors must return `*T` or `(*T, error)`
- No interface return types from constructors
- No named/tagged dependencies

## License

MIT
