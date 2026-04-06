package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/trofkm/compoapp"
)

// --- Config ---
// no lifecycle, just a plain value

type Config struct {
	DSN  string
	Port string
}

func NewConfig() *Config {
	return &Config{
		DSN:  "postgres://localhost:5432/myapp",
		Port: ":8080",
	}
}

// --- Logger ---
// has Init, no Start/Ready (simple component)

type Logger struct{}

func NewLogger() *Logger {
	return &Logger{}
}

func (l *Logger) Init(ctx context.Context) error {
	fmt.Println("[Logger] Init: setting up log output")
	time.Sleep(50 * time.Millisecond)
	return nil
}

func (l *Logger) Log(msg string) {
	fmt.Println("[LOG]", msg)
}

// --- Database ---
// depends on Config, Logger
// has Init + Start + Ready

type Database struct {
	config *Config
	logger *Logger
	ready  chan struct{}
}

func NewDatabase(cfg *Config, log *Logger) *Database {
	return &Database{
		config: cfg,
		logger: log,
		ready:  make(chan struct{}),
	}
}

func (d *Database) Init(ctx context.Context) error {
	d.logger.Log(fmt.Sprintf("Database] Init: connecting to %s", d.config.DSN))
	time.Sleep(100 * time.Millisecond)
	return nil
}

func (d *Database) Start(ctx context.Context) error {
	go func() {
		d.logger.Log("[Database] Start: running connection pool")
		time.Sleep(200 * time.Millisecond)
		d.logger.Log("[Database] Ready")
		close(d.ready)

		<-ctx.Done()
		d.logger.Log("[Database] shutting down")
	}()
	return nil
}

func (d *Database) Ready() <-chan struct{} {
	return d.ready
}

// --- Cache ---
// depends on Config, Logger
// has Init + Start + Ready

type Cache struct {
	config *Config
	logger *Logger
	ready  chan struct{}
}

func NewCache(cfg *Config, log *Logger) *Cache {
	return &Cache{
		config: cfg,
		logger: log,
		ready:  make(chan struct{}),
	}
}

func (c *Cache) Init(ctx context.Context) error {
	c.logger.Log("[Cache] Init: warming up")
	time.Sleep(80 * time.Millisecond)
	return nil
}

func (c *Cache) Start(ctx context.Context) error {
	go func() {
		c.logger.Log("[Cache] Start: connecting to redis")
		time.Sleep(150 * time.Millisecond)
		c.logger.Log("[Cache] Ready")
		close(c.ready)

		<-ctx.Done()
		c.logger.Log("[Cache] shutting down")
	}()
	return nil
}

func (c *Cache) Ready() <-chan struct{} {
	return c.ready
}

// --- UserRepository ---
// depends on Database, Cache
// has Start + Ready (no Init)

type UserRepository struct {
	db     *Database
	cache  *Cache
	logger *Logger
	ready  chan struct{}
}

func NewUserRepository(db *Database, cache *Cache, log *Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		cache:  cache,
		logger: log,
		ready:  make(chan struct{}),
	}
}

func (r *UserRepository) Start(ctx context.Context) error {
	go func() {
		r.logger.Log("[UserRepository] Start: preparing queries")
		time.Sleep(50 * time.Millisecond)
		r.logger.Log("[UserRepository] Ready")
		close(r.ready)

		<-ctx.Done()
		r.logger.Log("[UserRepository] shutting down")
	}()
	return nil
}

func (r *UserRepository) Ready() <-chan struct{} {
	return r.ready
}

// --- AuthService ---
// depends on UserRepository, Cache
// has Start + Ready

type AuthService struct {
	repo   *UserRepository
	cache  *Cache
	logger *Logger
	ready  chan struct{}
}

func NewAuthService(repo *UserRepository, cache *Cache, log *Logger) *AuthService {
	return &AuthService{
		repo:   repo,
		cache:  cache,
		logger: log,
		ready:  make(chan struct{}),
	}
}

func (a *AuthService) Start(ctx context.Context) error {
	go func() {
		a.logger.Log("[AuthService] Start: loading JWT keys")
		time.Sleep(100 * time.Millisecond)
		a.logger.Log("[AuthService] Ready")
		close(a.ready)

		<-ctx.Done()
		a.logger.Log("[AuthService] shutting down")
	}()
	return nil
}

func (a *AuthService) Ready() <-chan struct{} {
	return a.ready
}

// --- MetricsCollector ---
// depends on Logger only
// has Start, no Ready (fire and forget background worker)

type MetricsCollector struct {
	logger *Logger
}

func NewMetricsCollector(log *Logger) *MetricsCollector {
	return &MetricsCollector{logger: log}
}

func (m *MetricsCollector) Start(ctx context.Context) error {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		m.logger.Log("[MetricsCollector] Start: collecting metrics")
		for {
			select {
			case <-ticker.C:
				m.logger.Log("[MetricsCollector] tick: collected metrics")
			case <-ctx.Done():
				m.logger.Log("[MetricsCollector] shutting down")
				return
			}
		}
	}()
	return nil
}

// --- HTTPServer ---
// depends on AuthService, UserRepository, Config
// has Start + Ready — the root component

type HTTPServer struct {
	auth   *AuthService
	repo   *UserRepository
	config *Config
	logger *Logger
	ready  chan struct{}
}

func NewHTTPServer(auth *AuthService, repo *UserRepository, cfg *Config, log *Logger) *HTTPServer {
	return &HTTPServer{
		auth:   auth,
		repo:   repo,
		config: cfg,
		logger: log,
		ready:  make(chan struct{}),
	}
}

func (s *HTTPServer) Start(ctx context.Context) error {
	go func() {
		s.logger.Log(fmt.Sprintf("[HTTPServer] Start: listening on %s", s.config.Port))
		time.Sleep(100 * time.Millisecond)
		s.logger.Log("[HTTPServer] Ready")
		close(s.ready)

		<-ctx.Done()
		s.logger.Log("[HTTPServer] graceful shutdown")
	}()
	return nil
}

func (s *HTTPServer) Ready() <-chan struct{} {
	return s.ready
}

// --- main ---

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	container := compoapp.NewContainer()
	container.Debug()

	container.MustProvide(NewConfig)
	container.MustProvide(NewLogger)
	container.MustProvide(NewDatabase)
	container.MustProvide(NewCache)
	container.MustProvide(NewUserRepository)
	container.MustProvide(NewAuthService)
	container.MustProvide(NewMetricsCollector)
	container.MustProvide(NewHTTPServer)

	var server *HTTPServer
	if err := container.ResolveLifecycle(&server).Execute(ctx); err != nil {
		fmt.Println("fatal:", err)
		os.Exit(1)
	}
}
