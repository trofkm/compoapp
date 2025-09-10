package compoapp_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/trofkm/compoapp"
)

// Test types
type Database struct {
	Host string
}

type Cache struct {
	Host string
}

type Config struct {
	Port int
}

type UserService struct {
	db    *Database
	cache *Cache
}

type AuthService struct {
	db *Database
}

type Server struct {
	userService *UserService
	authService *AuthService
	config      *Config
}

type Application struct {
	server *Server
}

// Constructor functions
func NewDatabase() *Database {
	return &Database{Host: "localhost:5432"}
}

func NewCache() *Cache {
	return &Cache{Host: "localhost:6379"}
}

func NewConfig() *Config {
	return &Config{Port: 8080}
}

func NewUserService(db *Database, cache *Cache) *UserService {
	return &UserService{db: db, cache: cache}
}

func NewAuthService(db *Database) *AuthService {
	return &AuthService{db: db}
}

func NewServer(userService *UserService, authService *AuthService, config *Config) *Server {
	return &Server{
		userService: userService,
		authService: authService,
		config:      config,
	}
}

func NewApplication(server *Server) *Application {
	return &Application{server: server}
}

// Error constructor
func NewErrorDatabase() (*Database, error) {
	return nil, errors.New("database connection failed")
}

func NewSuccessfulDatabase() (*Database, error) {
	return &Database{Host: "success:5432"}, nil
}

// Interface example
type Storage interface {
	Save(data string) error
}

type FileStorage struct {
	path string
}

func (f *FileStorage) Save(data string) error {
	return nil
}

func NewFileStorage() *FileStorage {
	return &FileStorage{path: "/tmp"}
}

type DataProcessor struct {
	storage Storage
}

func NewDataProcessor(storage Storage) *DataProcessor {
	return &DataProcessor{storage: storage}
}

var _ = Describe("Container", func() {
	var container *compoapp.Container

	BeforeEach(func() {
		container = compoapp.NewContainer()
	})

	Describe("Basic Dependency Resolution", func() {
		It("should resolve simple dependencies", func() {
			Expect(container.Provide(NewDatabase)).To(Succeed())
			Expect(container.Provide(NewCache)).To(Succeed())
			Expect(container.Provide(NewConfig)).To(Succeed())
			Expect(container.Provide(NewUserService)).To(Succeed())
			Expect(container.Provide(NewAuthService)).To(Succeed())
			Expect(container.Provide(NewServer)).To(Succeed())
			Expect(container.Provide(NewApplication)).To(Succeed())

			var app *Application
			Expect(container.Resolve(&app)).To(Succeed())
			Expect(app).ToNot(BeNil())
			Expect(app.server).ToNot(BeNil())
			Expect(app.server.userService).ToNot(BeNil())
			Expect(app.server.authService).ToNot(BeNil())
			Expect(app.server.config).ToNot(BeNil())
			Expect(app.server.userService.db).ToNot(BeNil())
			Expect(app.server.userService.cache).ToNot(BeNil())
			Expect(app.server.authService.db).ToNot(BeNil())
		})
		// todo: this is pending, because based on type name these structs are equal.
		PIt("should resolve anonimous structs as dependencies in correct order", func() {
			callOrder := []string{}

			// Create constructors that record call order
			newA := func() *struct{ name string } {
				callOrder = append(callOrder, "A")
				return &struct{ name string }{name: "A"}
			}

			newB := func(a *struct{ name string }) *struct{ name string } {
				callOrder = append(callOrder, "B")
				return &struct{ name string }{name: "B"}
			}

			newC := func(b *struct{ name string }) *struct{ name string } {
				callOrder = append(callOrder, "C")
				return &struct{ name string }{name: "C"}
			}

			Expect(container.Provide(newA)).To(Succeed())
			Expect(container.Provide(newB)).To(Succeed())
			Expect(container.Provide(newC)).To(Succeed())

			var c *struct{ name string }
			Expect(container.Resolve(&c)).To(Succeed())

			// Should be called in dependency order: A -> B -> C
			Expect(callOrder).To(Equal([]string{"A", "B", "C"}))
		})

		It("should resolve dependencies in correct order", func() {
			callOrder := []string{}

			type A struct{}
			type B struct{}
			type C struct{}

			// Create constructors that record call order
			newA := func() *A {
				callOrder = append(callOrder, "A")
				return &A{}
			}

			newB := func(a *A) *B {
				callOrder = append(callOrder, "B")
				return &B{}
			}

			newC := func(b *B) *C {
				callOrder = append(callOrder, "C")
				return &C{}
			}

			Expect(container.Provide(newA)).To(Succeed())
			Expect(container.Provide(newB)).To(Succeed())
			Expect(container.Provide(newC)).To(Succeed())

			var c *C
			Expect(container.Resolve(&c)).To(Succeed())

			// Should be called in dependency order: A -> B -> C
			Expect(callOrder).To(Equal([]string{"A", "B", "C"}))
		})
	})

	Describe("Interface Resolution", func() {
		It("should resolve interface dependencies", func() {
			Expect(container.Provide(NewFileStorage)).To(Succeed())
			Expect(container.Provide(NewDataProcessor)).To(Succeed())

			var processor *DataProcessor
			Expect(container.Resolve(&processor)).To(Succeed())
			Expect(processor).ToNot(BeNil())
			Expect(processor.storage).ToNot(BeNil())

			// Verify it's the correct concrete type
			fs, ok := processor.storage.(*FileStorage)
			Expect(ok).To(BeTrue())
			Expect(fs.path).To(Equal("/tmp"))
		})
	})

	Describe("Error Handling", func() {
		PIt("should handle constructor errors", func() {
			Expect(container.Provide(NewErrorDatabase)).To(Succeed())

			var db *Database
			Expect(container.Resolve(&db)).To(MatchError(ContainSubstring("database connection failed")))
		})

		PIt("should handle successful constructors with error return", func() {
			Expect(container.Provide(NewSuccessfulDatabase)).To(Succeed())

			var db *Database
			Expect(container.Resolve(&db)).To(Succeed())
			Expect(db).ToNot(BeNil())
			Expect(db.Host).To(Equal("success:5432"))
		})

		It("should return error for missing dependencies", func() {
			Expect(container.Provide(NewServer)).To(Succeed()) // Missing UserService, AuthService, Config

			var server *Server
			Expect(container.Resolve(&server)).To(MatchError(ContainSubstring("missing constructor for dependency type")))
		})

		PIt("should return error for invalid target", func() {
			Expect(container.Resolve(nil)).To(MatchError("target must be a non-nil pointer"))
			var notAPointer string
			Expect(container.Resolve(notAPointer)).To(MatchError("target must be a non-nil pointer"))
		})
	})

	Describe("Circular Dependencies", func() {
		It("should detect circular dependencies", func() {
			// Create circular dependency: A -> B -> C -> A
			type A struct{}
			type B struct{}
			type C struct{}

			newA := func(c *C) *A { return &A{} }
			newB := func(a *A) *B { return &B{} }
			newC := func(b *B) *C { return &C{} }

			Expect(container.Provide(newA)).To(Succeed())
			Expect(container.Provide(newB)).To(Succeed())
			Expect(container.Provide(newC)).To(Succeed())

			var a *A
			Expect(container.Resolve(&a)).To(MatchError(ContainSubstring("circular dependency detected")))
		})
	})

	Describe("MustResolve", func() {
		It("should panic on resolution error", func() {
			Expect(container.Provide(NewServer)).To(Succeed()) // Missing dependencies

			var server *Server
			Expect(func() {
				container.MustResolve(&server)
			}).To(Panic())
		})

		It("should resolve successfully when dependencies are met", func() {
			Expect(container.Provide(NewDatabase)).To(Succeed())
			Expect(container.Provide(NewCache)).To(Succeed())
			Expect(container.Provide(NewConfig)).To(Succeed())
			Expect(container.Provide(NewUserService)).To(Succeed())
			Expect(container.Provide(NewAuthService)).To(Succeed())
			Expect(container.Provide(NewServer)).To(Succeed())

			var server *Server
			Expect(func() {
				container.MustResolve(&server)
			}).ToNot(Panic())
			Expect(server).ToNot(BeNil())
		})
	})

	Describe("Thread Safety", func() {
		It("should be thread-safe", func() {
			// This is a basic test - in practice you'd want more comprehensive concurrency testing
			Expect(container.Provide(NewDatabase)).To(Succeed())
			Expect(container.Provide(NewCache)).To(Succeed())
			Expect(container.Provide(NewConfig)).To(Succeed())
			Expect(container.Provide(NewUserService)).To(Succeed())
			Expect(container.Provide(NewAuthService)).To(Succeed())
			Expect(container.Provide(NewServer)).To(Succeed())

			var server1, server2 *Server
			done1 := make(chan bool)
			done2 := make(chan bool)

			go func() {
				defer GinkgoRecover()
				Expect(container.Resolve(&server1)).To(Succeed())
				done1 <- true
			}()

			go func() {
				defer GinkgoRecover()
				Expect(container.Resolve(&server2)).To(Succeed())
				done2 <- true
			}()

			<-done1
			<-done2

			Expect(server1).ToNot(BeNil())
			Expect(server2).ToNot(BeNil())
		})
	})
})
