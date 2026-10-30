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
	"net/http/httptest"
	"os"
	"path"
	"strings"

	"github.com/incurl/wikirevele/internal/bindings"
	"github.com/incurl/wikirevele/internal/console"
	"github.com/incurl/wikirevele/internal/etl"
	"github.com/incurl/wikirevele/internal/etlhistory"
	"github.com/incurl/wikirevele/internal/etlpipeline"
	"github.com/incurl/wikirevele/internal/jobs"
	"github.com/incurl/wikirevele/internal/pocketbase"
	"github.com/incurl/wikirevele/internal/runner"
	"github.com/incurl/wikirevele/internal/wikidata"
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

	// The progress of an ETL run.
	application.RegisterEvent[etlpipeline.Progress]("etl:progress")
}

func main() {
	// The binary has two modes, and they never run together.
	//
	// Without a subcommand the desktop application starts. With the
	// "etl" subcommand the program is the extraction worker, and it
	// does nothing but read a dump and write the two datasets. The
	// worker never initialises Wails, never opens the database and
	// never draws a window.
	if len(os.Args) > 1 && os.Args[1] == "etl" {
		if err := runETL(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "wikirevele:", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// runETL runs the extraction worker.
//
// The worker is the "etl" subcommand of the binary. The same code
// serves the desktop application, which spawns the binary as a child
// process, and the terminal, where a user pipes a dump into it.
//
// The worker writes the machine readable events to the standard output
// and the human readable logs to the standard error, so that the two
// channels never mix.
func runETL(args []string) error {
	command := etl.NewCommand()
	command.SetArgs(args)
	command.SilenceUsage = true

	return command.Execute()
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

	// The Wikidata service imports the trimmed datasets into the
	// database and reads them back for the panel.
	//
	// The application does not process a dump: that is the job of the
	// standalone ETL command, which writes the NDJSON files this
	// service imports.
	wikidataService := wikidata.NewService(pbService.App())
	wikidataService.SetDataDir(dataDir())

	// The extraction pipeline. The supervisor owns the child
	// processes, and the history records the runs.
	//
	// A run that was going when the application last closed is marked
	// as failed, so that it does not stay in the history as a run
	// that never finished.
	etlHistory := etlhistory.NewService(pbService.App())
	if reconciled, err := etlHistory.Reconcile(); err != nil {
		consoleLog.Warn("etl", fmt.Sprintf(
			"%d ETL run(s) were interrupted by a restart and have been marked as failed",
			reconciled))
	}

	etlSupervisor := etlpipeline.NewSupervisor()
	etlService := etlpipeline.NewService(etlSupervisor, etlHistory, wikidataService)

	appService := bindings.NewAppService(pbService, settings, jobManager, consoleLog, wikidataService, etlService)

	// Mount PocketBase's own REST and file API alongside the static
	// frontend. This keeps everything on a single origin: no extra
	// port, no separate PocketBase process and no CORS handling.
	pbHandler, handlerErr := pbService.Handler()
	if handlerErr != nil {
		return fmt.Errorf("create pocketbase handler: %w", handlerErr)
	}

	// The index is the answer to every route of the frontend, so it is
	// read once, here.
	index, err := readIndex()
	if err != nil {
		return err
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
			Handler: combineHandlers(
				pbHandler,
				spaHandler(application.AssetFileServerFS(assets), index),
			),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		// The running jobs are stopped when the application closes, so
		// that a job is never killed in the middle of a write. The
		// import is stopped too, so that it is never writing to a
		// database that is going away.
		OnShutdown: func() {
			jobManager.ShutdownWithTimeout(jobs.ShutdownTimeout)
			wikidataService.Close()
			// The pipelines are stopped, so that no decompressor
			// and no worker is left running after the window is
			// gone.
			etlSupervisor.Close()
		},
	})

	// The event bridge pushes job updates to the frontend.
	bridge = bindings.NewEventBridge(app)
	jobManager.SetEmitter(bridge)
	consoleLog.SetEmitter(bridge)
	wikidataService.SetProgressEmitter(bridge.EmitWikidataImportProgress)
	etlSupervisor.SetEmitter(bridge)
	etlSupervisor.SetLogSink(bridge)

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
//
// The frontend has routes of its own, so a path that is not a file is
// the frontend's page and not a missing file. The static handler
// answers those with the index, and the router on the client picks
// the page. Without this, a link to /wikidata is a 404 in server
// mode, and a reload of the page loses it.
func combineHandlers(apiHandler, staticHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiHandler.ServeHTTP(w, r)
			return
		}
		staticHandler.ServeHTTP(w, r)
	})
}

// spaHandler serves the index for the paths that are not files.
//
// The frontend is a single page application: its routes are paths that
// no file matches. The handler asks the static handler first, and when
// it answers with a 404 the index is served instead, so that the
// router on the client picks the page.
//
// A path that names a file is left alone: a missing asset stays a 404,
// because a silent index would hide a broken build and would answer a
// script request with HTML.
//
// The response of the static handler is buffered, because a handler
// that has written its header cannot change its status afterwards.
func spaHandler(staticHandler http.Handler, index []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder := httptest.NewRecorder()
		staticHandler.ServeHTTP(recorder, r)

		if recorder.Code != http.StatusNotFound || namesAFile(r.URL.Path) {
			// The file was found, or the path names one: the
			// recorded response is the answer, headers and body
			// alike.
			for name, values := range recorder.Header() {
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes())
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	})
}

// namesAFile tells whether a path names a file rather than a route.
//
// The routes of the frontend have no extension: /wikidata and
// /wikidata/Q90 are pages, /assets/app.js is a file. The extension is
// the difference, and it is what tells a missing asset from a route.
func namesAFile(urlPath string) bool {
	base := path.Base(urlPath)
	return path.Ext(base) != ""
}

// readIndex reads the index of the frontend from the embedded assets.
//
// The index is read once, when the application starts: it is the
// fallback of every route, and reading it per request would be a waste.
func readIndex() ([]byte, error) {
	index, err := assets.ReadFile("frontend/dist/index.html")
	if err != nil {
		return nil, fmt.Errorf("read the frontend index: %w", err)
	}
	return index, nil
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
