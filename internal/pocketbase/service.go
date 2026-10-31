// Package pocketbase embeds PocketBase as a Go library and owns the
// Wikirevele persistence layer.
//
// PocketBase is never run as a standalone process. The application
// bootstraps the library, runs the Go migrations in the migrations
// subpackage and mounts PocketBase's own REST and file API onto the
// Wails asset server, so that every file the app stores is served from
// the same origin as the UI.
//
// The schema itself is defined by the Go migrations, which PocketBase
// tracks and applies. This file holds only the service plumbing and the
// names shared with the rest of the application.
package pocketbase

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"

	// Importing the migrations package registers the Wikirevele schema
	// migrations with PocketBase's own migration list.
	_ "github.com/incurl/wikirevele/internal/pocketbase/migrations"
)

// Service owns the embedded PocketBase application.
type Service struct {
	app *pocketbase.PocketBase
	// dataDir is the directory holding the SQLite database and the
	// uploaded files.
	dataDir string
}

// NewService creates the PocketBase service rooted at dataDir.
//
// The database and the uploaded files live side by side in dataDir,
// which defaults to "pb_data" next to the executable.
func NewService(dataDir string) *Service {
	if dataDir == "" {
		dataDir = DefaultDataDir()
	}
	return &Service{dataDir: dataDir}
}

// DefaultDataDir returns the default location of the Wikirevele data
// directory.
func DefaultDataDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "pb_data")
	}
	return "pb_data"
}

// App returns the underlying PocketBase application.
func (s *Service) App() *pocketbase.PocketBase { return s.app }

// DataDir returns the directory holding the database and the files.
func (s *Service) DataDir() string { return s.dataDir }

// Bootstrap initialises PocketBase and applies the Wikirevele migrations.
//
// PocketBase is used purely as a library: Start() is never called, so
// no HTTP server is bound and no separate process is required.
func (s *Service) Bootstrap() error {
	if err := os.MkdirAll(s.dataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	s.app = pocketbase.NewWithConfig(pocketbase.Config{
		DefaultDataDir: s.dataDir,
	})

	if err := s.app.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap pocketbase: %w", err)
	}

	// Apply the Go migrations that create the Wikirevele schema.
	if err := s.app.RunAppMigrations(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}

// Handler returns the PocketBase HTTP handler.
//
// The returned handler serves PocketBase's REST API and, crucially, its
// native file route (/api/files/<collection>/<record>/<filename>).
// Wikirevele mounts it on the Wails asset server so that stored files are
// served from the same origin as the UI, without opening a second port.
func (s *Service) Handler() (http.Handler, error) {
	if s.app == nil {
		return nil, fmt.Errorf("pocketbase: service has not been bootstrapped")
	}
	router, err := apis.NewRouter(s.app)
	if err != nil {
		return nil, fmt.Errorf("create pocketbase router: %w", err)
	}
	handler, err := router.BuildMux()
	if err != nil {
		return nil, fmt.Errorf("build pocketbase mux: %w", err)
	}
	return handler, nil
}

// Close releases the resources PocketBase holds: the database
// connections and the cron ticker.
//
// It is only needed in tests and in the server mode, where the process
// outlives the request that created the service.
func (s *Service) Close() error {
	if s.app == nil {
		return nil
	}
	return s.app.ResetBootstrapState()
}

// Collection returns a collection by name.
func (s *Service) Collection(name string) (*core.Collection, error) {
	if s.app == nil {
		return nil, fmt.Errorf("pocketbase: service has not been bootstrapped")
	}
	col, err := s.app.FindCollectionByNameOrId(name)
	if err != nil {
		return nil, fmt.Errorf("find collection %q: %w", name, err)
	}
	return col, nil
}
