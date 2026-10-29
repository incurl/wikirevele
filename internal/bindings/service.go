// Package bindings exposes the Valkyrie backend to the frontend as
// Wails services.
//
// These are the primary communication channel between the React UI and
// Go. The frontend never talks to PocketBase or to a remote API
// directly.
package bindings

import (
	"github.com/incurl/valkyrie/internal/console"
	"github.com/incurl/valkyrie/internal/jobs"
	"github.com/incurl/valkyrie/internal/pocketbase"
)

// AppService is the main Wails service. It is bound to the frontend and
// exposes every operation the UI needs.
type AppService struct {
	pb       *pocketbase.Service
	settings *pocketbase.SettingsStore
	jobs     *jobs.Manager
	console  *console.Log
}

// NewAppService creates the application service.
func NewAppService(
	pb *pocketbase.Service,
	settings *pocketbase.SettingsStore,
	jobManager *jobs.Manager,
	log *console.Log,
) *AppService {
	return &AppService{pb: pb, settings: settings, jobs: jobManager, console: log}
}

// The service deliberately exposes no accessors for the objects it
// owns. Every exported method on a bound service becomes a method in the
// generated TypeScript, so a JobManager() or a Settings() accessor
// advertises an API the frontend cannot use: those types hold unexported
// fields and serialise to an empty object. They were also unused.
//
// If a method is needed, bind it explicitly here.

// ListConsoleEntries returns the recorded debug console entries,
// oldest first.
func (s *AppService) ListConsoleEntries() []console.Entry {
	if s.console == nil {
		return nil
	}
	return s.console.Entries()
}

// ClearConsoleEntries discards the recorded debug console entries.
func (s *AppService) ClearConsoleEntries() {
	if s.console == nil {
		return
	}
	s.console.Clear()
}

// SettingsView is the settings as the frontend sees them.
//
// The API key is never included: the frontend only learns whether one
// is configured.
type SettingsView struct {
	// HasAPIKey reports whether an API key is stored.
	HasAPIKey bool `json:"has_api_key"`
	// BaseURL is the configured remote API host.
	BaseURL string `json:"base_url"`
	// UI holds the interface preferences.
	UI pocketbase.UI `json:"ui"`
}

// GetSettings returns the application settings.
func (s *AppService) GetSettings() (SettingsView, error) {
	view := SettingsView{}

	if _, found, err := s.settings.GetAPIKey(); err != nil {
		return SettingsView{}, err
	} else {
		view.HasAPIKey = found
	}

	baseURL, err := s.settings.GetBaseURL()
	if err != nil {
		return SettingsView{}, err
	}
	view.BaseURL = baseURL

	ui, err := s.settings.GetUI()
	if err != nil {
		return SettingsView{}, err
	}
	view.UI = ui

	return view, nil
}

// SaveCredentials stores the API credentials.
//
// An empty apiKey keeps the stored key, so the frontend never has to
// read back a secret it cannot see.
func (s *AppService) SaveCredentials(apiKey, baseURL string) error {
	if apiKey != "" {
		if err := s.settings.SetAPIKey(apiKey); err != nil {
			return err
		}
	}
	if baseURL != "" {
		if err := s.settings.SetBaseURL(baseURL); err != nil {
			return err
		}
	}
	return nil
}

// SaveUI stores the interface preferences, including the zoom level.
func (s *AppService) SaveUI(ui pocketbase.UI) error {
	return s.settings.SetUI(ui)
}

// SubmitJob queues a job.
func (s *AppService) SubmitJob(request jobs.Request) (jobs.Job, error) {
	return s.jobs.Submit(request)
}

// CancelJob cancels a pending or running job.
func (s *AppService) CancelJob(jobID string) error {
	return s.jobs.Cancel(jobID)
}

// DeleteJob removes a finished job from the queue.
//
// The artifacts the job produced are removed with it.
func (s *AppService) DeleteJob(jobID string) error {
	return s.jobs.DeleteJob(jobID)
}

// RetryJob re-runs a job with the same parameters.
//
// Only the **inputs** of the original job are re-used. Its outputs stay
// with the original job: a retry produces its own results, and moving
// the outputs would strip the original of them.
func (s *AppService) RetryJob(jobID string) (jobs.Job, error) {
	job, err := s.jobs.GetJob(jobID)
	if err != nil {
		return jobs.Job{}, err
	}

	request := jobs.Request{
		Title:            job.Title,
		Prompt:           job.Prompt,
		Params:           job.Params,
		InputArtifactIDs: job.InputArtifactIDs,
	}
	return s.jobs.Submit(request)
}

// ListJobs returns the recorded jobs.
func (s *AppService) ListJobs(limit int) ([]jobs.Job, error) {
	return s.jobs.ListJobs(limit)
}

// ListArtifacts returns the stored artifacts.
func (s *AppService) ListArtifacts(jobID string, limit int) ([]jobs.Artifact, error) {
	return s.jobs.ListArtifacts(jobID, limit)
}

// DeleteArtifact removes a stored artifact.
func (s *AppService) DeleteArtifact(artifactID string) error {
	return s.jobs.DeleteArtifact(artifactID)
}

// AppName is the name of the application.
const AppName = "Valkyrie"

// AppVersion is the version of the application.
const AppVersion = "0.1.0"
