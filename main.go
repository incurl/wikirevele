// Valkyrie is a desktop application scaffold.
//
// It wires together Wails v3, an embedded PocketBase and a React
// frontend, and it provides a generic job queue that an application
// plugs its own backend into.
//
// The UI shell is deliberately plain: it is the starting point, not
// the product. Replace the contents of internal/runner with your own
// work and extend the frontend as needed.
//
// Persistence, file storage, migrations and access rules are provided
// by PocketBase, embedded as a Go library. PocketBase is never run as a
// standalone process: its REST and file API is mounted on the Wails
// asset server so the files are served from the UI's own origin.
package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/incurl/valkyrie/internal/bindings"
	"github.com/incurl/valkyrie/internal/console"
	"github.com/incurl/valkyrie/internal/jobs"
	"github.com/incurl/valkyrie/internal/pocketbase"
	"github.com/incurl/valkyrie/internal/runner"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails embeds the built frontend. Any file in frontend/dist is served
// to the webview.
//
//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// Register the job update event so the binding generator produces a
	// strongly typed TS API for it.
	application.RegisterEvent[jobs.Job]("job:updated")

	// The debug console entry event, likewise.
	application.RegisterEvent[console.Entry]("console:entry")
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run bootstraps the backend and starts the Wails application.
func run() error {
	// --- PocketBase ------------------------------------------------
	pbService := pocketbase.NewService(dataDir())
	if err := pbService.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap pocketbase: %w", err)
	}

	cipher, err := pocketbase.NewCipher(pbService.DataDir())
	if err != nil {
		return fmt.Errorf("initialise settings encryption: %w", err)
	}
	settings := pocketbase.NewSettingsStore(pbService.App(), cipher)

	// The event bridge pushes job updates to the frontend. It is
	// created once the Wails application exists, so it is declared here
	// and assigned later.
	var bridge *bindings.EventBridge

	// The debug console records the application traffic for the panel
	// at the bottom of the window.
	consoleLog := console.New()

	// The job manager delegates the work to the registered runner. The
	// scaffold ships an echo runner, which is replaced by your own.
	jobManager := jobs.NewManager(
		pbService.App(),
		jobs.WithLog(consoleLog),
		jobs.WithRunnerFunc(func() jobs.Runner {
			return runner.NewEcho(settings, consoleLog)
		}),
	)

	// A job that was running when the application last closed was
	// never finished and never failed. Nothing would ever pick it up
	// again, and the user could not even delete it. The reconciliation
	// marks it as failed, so it appears in the history and can be
	// cleared.
	if reconciled, err := jobManager.Reconcile(); err != nil {
		return fmt.Errorf("reconcile the interrupted jobs: %w", err)
	} else if reconciled > 0 {
		consoleLog.Warn("jobs", fmt.Sprintf(
			"%d job(s) were interrupted by a restart and have been marked as failed", reconciled))
	}

	appService := bindings.NewAppService(pbService, settings, jobManager, consoleLog)

	// Mount PocketBase's own REST and file API alongside the static
	// frontend. This keeps everything on a single origin: no extra
	// port, no separate PocketBase process and no CORS handling.
	pbHandler, handlerErr := pbService.Handler()
	if handlerErr != nil {
		return fmt.Errorf("create pocketbase handler: %w", handlerErr)
	}

	// --- Wails application -----------------------------------------
	app := application.New(application.Options{
		Name: bindings.AppName,
		// The description lands in the about box of the platform, so
		// it is the one place the version is visible without a menu.
		Description: "A Wails v3 + PocketBase + React desktop application, version " + bindings.AppVersion,
		Services: []application.Service{
			application.NewService(appService),
		},
		Assets: application.AssetOptions{
			Handler: combineHandlers(pbHandler, application.AssetFileServerFS(assets)),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// The running jobs are stopped when the application closes, so
		// that a job is never killed in the middle of a write.
		OnShutdown: func() {
			jobManager.ShutdownWithTimeout(jobs.ShutdownTimeout)
		},
	})

	// The event bridge pushes job updates to the frontend.
	bridge = bindings.NewEventBridge(app)
	jobManager.SetEmitter(bridge)
	consoleLog.SetEmitter(bridge)

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  bindings.AppName,
		Width:  1440,
		Height: 900,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	return app.Run()
}

// combineHandlers routes /api/* to the PocketBase handler and
// everything else to the static frontend.
func combineHandlers(apiHandler, staticHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		staticHandler.ServeHTTP(w, r)
	})
}

// dataDir returns the directory holding the database and the uploaded
// files.
//
// The VALKYRIE_DATA_DIR environment variable overrides the default,
// which lets the app run against an isolated database during development
// and testing.
func dataDir() string {
	if dir := os.Getenv("VALKYRIE_DATA_DIR"); dir != "" {
		return dir
	}
	return pocketbase.DefaultDataDir()
}
