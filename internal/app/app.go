package app

import (
	"fmt"

	"github.com/evanjhopkins/RunBinder/internal/platform"
	"github.com/evanjhopkins/RunBinder/internal/store"
)

type OpenRepository func() (store.Repository, error)

type Application struct {
	Definitions *Definitions
	Tasks       *Tasks
	Service     *Service
}

func New(paths platform.Paths) *Application {
	definitions := &Definitions{}
	openRepository := func() (store.Repository, error) {
		if err := platform.EnsureStorage(paths); err != nil {
			return nil, fmt.Errorf("create storage directory: %w", err)
		}
		return store.OpenSQLite(paths.Database)
	}
	return &Application{
		Definitions: definitions,
		Tasks:       &Tasks{paths: paths, definitions: definitions, openRepository: openRepository},
		Service:     &Service{paths: paths, openRepository: openRepository},
	}
}
