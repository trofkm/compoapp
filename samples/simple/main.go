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
type OtherServer struct {
	storage *Storage
}

type App struct {
	server *Server
	other  *OtherServer
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
func NewOtherServer(storage *Storage) *OtherServer {
	fmt.Println("Creating Other Server with Storage")
	return &OtherServer{storage: storage}
}

func NewApp(server *Server, other *OtherServer) (*App, error) {
	fmt.Println("Creating App with Server")
	return &App{server, other}, nil
}

// Usage
func main() {
	container := compoapp.NewContainer()

	// Register constructors
	container.MustProvide(NewStorage)
	container.MustProvide(NewOtherServer)
	container.MustProvide(NewServer)
	container.MustProvide(NewApp)

	// Resolve dependencies
	var app *App
	container.MustResolve(&app)

	// app is now fully constructed with all dependencies!
	fmt.Printf("App created: %+v\n", app)
}
