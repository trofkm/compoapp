package main

import "github.com/trofkm/compoapp"

type IStorage interface {
	Get(id string) string
}

type Storage struct{}

var _ IStorage = (*Storage)(nil)

func NewStorage() *Storage {
	return &Storage{}
}

func (s *Storage) Get(id string) string {
	return "this is storage"
}

type Server struct {
	storage IStorage
}

func NewServer(storage IStorage) *Server {
	return &Server{
		storage,
	}
}

type App struct {
	server *Server
}

func NewApp(server *Server) *App {
	return &App{server}
}

func main() {
	container := compoapp.NewContainer()
	container.Debug()

	container.MustProvide(NewStorage)
	container.MustProvide(NewServer)
	container.MustProvide(NewApp)

	var app *App
	container.MustResolve(&app)

	if err := container.Visualize("graph.dot"); err != nil {
		panic(err)
	}
}
