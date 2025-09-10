# 📦 CompoApp - Lightweight DI Framework for Go

**CompoApp** is a zero-dependency, 300-line DI (Dependency Injection) framework for Go that makes building scalable applications easy. It automatically resolves dependencies, manages component lifecycle, and handles graceful shutdowns.

## 🌟 Features

- **Zero Dependencies** - Pure Go, no external libraries
- **Ultra Lightweight** - Only ~300 lines of clean, readable code
- **Automatic Dependency Resolution** - Register constructors, we handle the rest
- **Type-Based Wiring** - Dependencies resolved by function parameter types
- **Topological Sorting** - Components created in correct dependency order
- **Circular Dependency Detection** - Prevents runtime deadlocks
- **Thread-Safe** - Safe for concurrent use
- **Context-Based Lifecycle** - Graceful startup/shutdown

## 🚀 Quick Start

```go
package main

import "github.com/trofkm/compoapp"

// Define your types
type Database struct {
    host string
}

type UserService struct {
    db *Database
}

type HTTPServer struct {
    userService *UserService
}

// Constructor functions
func NewDatabase() *Database {
    return &Database{host: "localhost:5432"}
}

func NewUserService(db *Database) *UserService {
    return &UserService{db: db}
}

func NewHTTPServer(userService *UserService) *HTTPServer {
    return &HTTPServer{userService: userService}
}

func main() {
    // Create container
    container := di.NewContainer()

    // Register constructors
    container.MustProvide(NewDatabase)
    container.MustProvide(NewUserService)
    container.MustProvide(NewHTTPServer)

    // Resolve dependencies automatically
    var server *HTTPServer
    container.MustResolve(&server)

    // server is now fully constructed with all dependencies!
    fmt.Printf("Server created with database: %s\n", server.userService.db.host)
}
```

## 🎯 How It Works

1. **Register Constructors** - Provide functions that create your components
2. **Automatic Analysis** - Container uses reflection to analyze parameters
3. **Dependency Graph** - Builds dependency relationships automatically
4. **Topological Sort** - Orders components for proper creation sequence
5. **Resolve Dependencies** - Container creates instances in correct order

## 🛠️ API Reference

```go
// Core functions
func NewContainer() *Container
func (c *Container) Provide(constructor interface{}) error
func (c *Container) MustProvide(constructor interface{})
func (c *Container) ProvideNamed(name string, constructor interface{}) error
func (c *Container) MustProvideNamed(name string, constructor interface{}) error
func (c *Container) Resolve(target interface{}) error
func (c *Container) MustResolve(target interface{})
```

## ⚠️ Current Limitations

- **Exact Type Matching Only** - No interface binding support
- **Basic Named Dependencies** - `ProvideNamed` exists but limited resolution
- **No Lifecycle Hooks** - Basic startup/shutdown only
- **Simple Error Handling** - Limited diagnostic information

## 🛣️ Roadmap

- [x] Basic dependency resolution with reflection
- [x] Topological sorting and circular dependency detection
- [x] Thread-safe container operations
- [ ] Interface binding support
- [ ] Named dependency resolution

## 📊 Benefits

### Clean Architecture
```go
// Instead of manual wiring:
db := NewDatabase()
cache := NewCache()
userService := NewUserService(db, cache)
authService := NewAuthService(userService)
server := NewServer(userService, authService)

// Use automatic resolution:
container.MustProvide(NewDatabase)
container.MustProvide(NewCache)
container.MustProvide(NewUserService)
container.MustProvide(NewAuthService)
container.MustProvide(NewServer)

var server *Server
container.MustResolve(&server)
```

## 📦 Installation

```bash
go get github.com/trofkm/compoapp
```

## 📄 License

MIT License - see LICENSE file for details.

---

*"300 lines of code that solve dependency injection elegantly"*

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
