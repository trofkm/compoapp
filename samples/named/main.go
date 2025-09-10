package main

import (
	"fmt"

	"github.com/trofkm/compoapp"
)

// Define your types
type Storage struct {
	// fields
}

type Server struct {
	storage *Storage
	// fields
}

type App struct {
	server *Server
	other  *Server
	// fields
}

// Define constructor functions
func NewStorage() *Storage {
	fmt.Println("Creating Storage")
	return &Storage{}
}

func NewServer(storage *Storage) *Server {
	fmt.Println("Creating Server with Storage")
	return &Server{storage: storage}
}

func NewApp(server *Server, other *Server) (*App, error) {
	fmt.Println("Creating App with Server")
	return &App{server, other}, nil
}

// Usage
// todo: doesn't work for now
func main() {
	container := compoapp.NewContainer()

	// Register constructors
	container.MustProvide(NewStorage)
	// todo this one should inject Server into App ctor as 'other' variable
	container.MustProvideNamed("other", NewServer)
	// todo this one should inject Server into App ctor as 'server' variable
	container.MustProvideNamed("server", NewServer)
	container.MustProvide(NewApp)

	// Resolve dependencies
	var app *App
	container.MustResolve(&app)

	// app is now fully constructed with all dependencies!
	fmt.Printf("App created: %+v\n", app)
}
