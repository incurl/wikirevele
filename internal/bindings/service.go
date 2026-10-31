// Package bindings exposes the WikiRevele backend to the frontend as
// Wails services.
//
// These are the primary communication channel between the React UI and
// Go. The frontend never talks to PocketBase or to a remote API
// directly.
package bindings

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/incurl/wikirevele/internal/console"
	"github.com/incurl/wikirevele/internal/etlhistory"
	"github.com/incurl/wikirevele/internal/etlpipeline"
	"github.com/incurl/wikirevele/internal/jobs"
	"github.com/incurl/wikirevele/internal/pocketbase"
	"github.com/incurl/wikirevele/internal/wikidata"
)

// AppService is the main Wails service. It is bound to the frontend and
// exposes every operation the UI needs.
type AppService struct {
	pb       *pocketbase.Service
	settings *pocketbase.SettingsStore
	jobs     *jobs.Manager
	console  *console.Log
	// wikidata is the service that imports the datasets and reads
	// them back. It may be nil, in which case the panel has nothing
	// to show.
	wikidata *wikidata.Service
	// etl runs the pipelines that produce the datasets.
	etl *etlpipeline.Service
}

// NewAppService creates the application service.
//
// The wikidataDir is the directory the Wikidata subset is read from. An
// empty directory means the panel has no subset to browse, which is the
// case when the application runs without one.
func NewAppService(
	pb *pocketbase.Service,
	settings *pocketbase.SettingsStore,
	jobManager *jobs.Manager,
	log *console.Log,
	wikidataService *wikidata.Service,
	etlService *etlpipeline.Service,
) *AppService {
	return &AppService{
		pb:       pb,
		settings: settings,
		jobs:     jobManager,
		console:  log,
		wikidata: wikidataService,
		etl:      etlService,
	}
}

// WikidataItemsView is the imported items as the frontend sees them.
//
// The type lives here rather than in the wikidata package, because a
// bound method must not return the type of an internal package: the
// frontend would receive it as an empty object.
type WikidataItemsView struct {
	// Items are the imported items.
	Items []WikidataItemView `json:"items"`
	// Total is the number of items in the dataset.
	Total int64 `json:"total"`
	// Properties are the imported properties, by identifier.
	//
	// The dictionary is small enough to send whole, and the panel
	// needs the names of the properties to label the claims.
	Properties map[string]string `json:"properties"`
}

// WikidataItemView is one imported item.
type WikidataItemView struct {
	// QID is the identifier of the item.
	QID string `json:"qid"`
	// Labels holds the labels of the item, by language.
	Labels map[string]string `json:"labels"`
	// Descriptions holds the descriptions of the item, by language.
	Descriptions map[string]string `json:"descriptions"`
	// Claims holds the statements of the item, by property.
	//
	// The statements are the raw JSON of the dataset: the panel has
	// its own types for them, and the backend does not have to mirror
	// those.
	Claims map[string]any `json:"claims"`
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

// GetWikidataItems returns the imported items the panel browses.
//
// The items are read from the database, where the importer put them.
// The panel therefore shows what was imported, and never anything that
// is still a file.
//
// An empty dataset is not an error: the panel shows an empty list.
func (s *AppService) GetWikidataItems(limit, offset int) (WikidataItemsView, error) {
	view := WikidataItemsView{
		Items:      []WikidataItemView{},
		Properties: map[string]string{},
	}

	if s.wikidata == nil {
		return view, nil
	}

	reader := wikidata.NewReader(s.wikidata.App())

	items, err := reader.ListItems(limit, offset)
	if err != nil {
		return view, err
	}

	total, err := reader.CountItems()
	if err != nil {
		return view, err
	}
	view.Total = total

	for _, item := range items {
		view.Items = append(view.Items, WikidataItemView{
			QID:          item.QID,
			Labels:       labelsOf(item.Labels),
			Descriptions: labelsOf(item.Descriptions),
			Claims:       claimsOf(item.Claims),
		})
	}

	// The property dictionary gives the panel the name of every
	// property, so that a claim is shown with its label and not only
	// with its identifier.
	properties, err := reader.ListProperties(0, 0)
	if err != nil {
		return view, err
	}
	for _, property := range properties {
		if label, ok := property.Labels["en"]; ok && label.Value != "" {
			view.Properties[property.PID] = label.Value
		}
	}

	return view, nil
}

// ImportWikidataFile imports a dataset into the database.
//
// The kind is "items" or "properties". The import runs in the
// background, and the progress is pushed to the frontend as a Wails
// event, so that the panel is not blocked while a large file is being
// imported.
//
// The records are upserted on their identifier, so that importing the
// same file twice does not duplicate them.
func (s *AppService) ImportWikidataFile(kind, path string) error {
	if s.wikidata == nil {
		return fmt.Errorf("wikidata: the importer is not available")
	}
	return s.wikidata.ImportFile(kind, path)
}

// ImportWikidataDirectory imports both datasets of a directory.
//
// The files are the ones the ETL worker writes. It is the way the
// panel imports a dataset that was produced elsewhere.
func (s *AppService) ImportWikidataDirectory(path string) error {
	if s.wikidata == nil {
		return fmt.Errorf("wikidata: the importer is not available")
	}
	return s.wikidata.ImportDirectory(path)
}

// WikidataClaimFilter is a filter on the value of one property, as the
// frontend sends it.
type WikidataClaimFilter struct {
	// Property is the property of the claim, such as "P131".
	Property string `json:"property"`
	// Value is the value the claim must hold, such as "Q90".
	Value string `json:"value"`
}

// WikidataPageRequest is a request for one page of the items, as the
// frontend sends it.
type WikidataPageRequest struct {
	// Limit is the number of items to return.
	Limit int `json:"limit"`
	// Offset is the number of items to skip.
	Offset int `json:"offset"`
	// SortField is the field to sort on.
	SortField string `json:"sort_field"`
	// SortDescending tells whether the order is reversed.
	SortDescending bool `json:"sort_descending"`
	// Filters are the claims the items must hold.
	Filters []WikidataClaimFilter `json:"filters"`
}

// WikidataPageView is one page of the items, as the frontend sees it.
type WikidataPageView struct {
	// Items are the items of the page.
	Items []WikidataItemView `json:"items"`
	// Total is the number of items that match the filters.
	Total int64 `json:"total"`
	// Properties are the imported properties, by identifier.
	Properties map[string]string `json:"properties"`
}

// GetWikidataItemsPage returns one page of the imported items.
//
// The filtering, the sorting and the counting happen in the database
// and not in the frontend: the panel shows a dataset of a whole city,
// and reading all of it to keep the twentieth part would not scale.
//
// The filters are on the claims of the items, because an item has no
// columns: its properties are claims.
func (s *AppService) GetWikidataItemsPage(request WikidataPageRequest) (WikidataPageView, error) {
	view := WikidataPageView{
		Items:      []WikidataItemView{},
		Properties: map[string]string{},
	}

	if s.wikidata == nil {
		return view, nil
	}

	reader := wikidata.NewReader(s.wikidata.App())

	filters := make([]wikidata.ClaimFilter, 0, len(request.Filters))
	for _, filter := range request.Filters {
		filters = append(filters, wikidata.ClaimFilter{
			Property: filter.Property,
			Value:    filter.Value,
		})
	}

	items, total, err := reader.ListItemsPage(wikidata.PageQuery{
		Limit:          request.Limit,
		Offset:         request.Offset,
		SortField:      request.SortField,
		SortDescending: request.SortDescending,
		Filters:        filters,
	})
	if err != nil {
		return view, err
	}

	view.Total = total

	for _, item := range items {
		view.Items = append(view.Items, WikidataItemView{
			QID:          item.QID,
			Labels:       labelsOf(item.Labels),
			Descriptions: labelsOf(item.Descriptions),
			Claims:       claimsOf(item.Claims),
		})
	}

	// The property dictionary gives the panel the name of every
	// property, so that a claim is shown with its label and not only
	// with its identifier.
	properties, err := reader.ListProperties(0, 0)
	if err != nil {
		return view, err
	}
	for _, property := range properties {
		if label, ok := property.Labels["en"]; ok && label.Value != "" {
			view.Properties[property.PID] = label.Value
		}
	}

	return view, nil
}

// GetWikidataDataDir returns the directory the datasets are read from.
//
// It is the data directory of the application, which is where the ETL
// command is expected to leave its output. The panel uses it to fill
// the path of the import, so that the user does not have to type it.
//
// The path is not a secret: it is the directory of the user's own
// data, and the application tells the user where its data is.
func (s *AppService) GetWikidataDataDir() string {
	if s.wikidata == nil {
		return ""
	}
	return s.wikidata.DataDir()
}

// GetWikidataImportProgress returns the state of the last import.
//
// The panel calls it when it starts, so that it shows the outcome of
// the import that ran before it was opened.
func (s *AppService) GetWikidataImportProgress() wikidata.ImportProgress {
	if s.wikidata == nil {
		return wikidata.ImportProgress{Done: true}
	}
	return s.wikidata.Progress()
}

// CancelWikidataImport stops the import that is running.
//
// The records that were written before the cancel stay written, and the
// import can be run again: the records are upserted.
func (s *AppService) CancelWikidataImport() {
	if s.wikidata == nil {
		return
	}
	s.wikidata.Cancel()
}

// EtlJobOptions are the parameters of a pipeline run, as the frontend
// sends them.
//
// The type is declared here rather than in the etlpipeline package,
// because a bound method must not take the type of an internal
// package: the generator would not know what it is.
type EtlJobOptions struct {
	// DumpPath is the path of the compressed dump.
	DumpPath string `json:"dump_path"`
	// TargetQID is the entity the items are linked to.
	TargetQID string `json:"target_qid"`
	// RecursiveP131 tells whether the administrative hierarchy is
	// walked.
	RecursiveP131 bool `json:"recursive_p131"`
	// Languages are the languages that are kept.
	Languages []string `json:"langs"`
	// OutItemsPath is the file the items are written to.
	OutItemsPath string `json:"out_items_path"`
	// OutPropsPath is the file the properties are written to.
	OutPropsPath string `json:"out_props_path"`
	// Overwrite tells whether an existing output file may be
	// replaced.
	Overwrite bool `json:"overwrite"`
}

// EtlProgress is the state of a pipeline run, as the frontend sees it.
type EtlProgress struct {
	// ID identifies the run.
	ID string `json:"id"`
	// Status is where the run is.
	Status string `json:"status"`
	// Command is the shell pipeline that is running.
	//
	// It is the command the user can copy and run again.
	Command string `json:"command"`
	// ProcessedLines is the number of lines the worker has read.
	ProcessedLines int64 `json:"processed_lines"`
	// MatchedItems is the number of items that were kept.
	MatchedItems int64 `json:"matched_items"`
	// CollectedProperties is the number of properties that were
	// written.
	CollectedProperties int64 `json:"collected_properties"`
	// SpeedMBs is the rate of the run.
	SpeedMBs float64 `json:"speed_mbs"`
	// StartedAt is when the run began.
	StartedAt string `json:"started_at"`
	// FinishedAt is when the run ended. It is empty while the run is
	// going.
	FinishedAt string `json:"finished_at"`
	// Error is why the run failed. It is empty when it did not.
	Error string `json:"error"`
	// ItemFile is the file the items were written to.
	ItemFile string `json:"item_file"`
	// PropFile is the file the properties were written to.
	PropFile string `json:"prop_file"`
}

// EtlHistoryEntry is one run of the worker, as the frontend sees it.
//
// The command is the whole point of the entry: it is what the user
// copies to run the job again.
type EtlHistoryEntry struct {
	// ID is the identifier of the record.
	ID string `json:"id"`
	// FullCommand is the shell pipeline that was run.
	FullCommand string `json:"full_command"`
	// TargetQID is the entity the items were linked to.
	TargetQID string `json:"target_qid"`
	// RecursiveP131 says whether the hierarchy was walked.
	RecursiveP131 bool `json:"recursive_p131"`
	// Langs are the languages that were kept.
	Langs string `json:"langs"`
	// DumpFilePath is the dump that was read.
	DumpFilePath string `json:"dump_file_path"`
	// OutItemsPath is the file the items were written to.
	OutItemsPath string `json:"out_items_path"`
	// OutPropsPath is the file the properties were written to.
	OutPropsPath string `json:"out_props_path"`
	// StartedAt is when the run began.
	StartedAt string `json:"started_at"`
	// FinishedAt is when the run ended. It is empty while the run is
	// going.
	FinishedAt string `json:"finished_at"`
	// Status is where the run is.
	Status string `json:"status"`
	// ErrorMessage is why the run failed. It is empty when it did not.
	ErrorMessage string `json:"error_message"`
	// Notes is the remark of the user.
	Notes string `json:"notes"`
}

// toEtlJobOptions turns the options of the frontend into the ones of
// the pipeline.
func toEtlJobOptions(opts EtlJobOptions) etlpipeline.JobOptions {
	return etlpipeline.JobOptions{
		DumpPath:      opts.DumpPath,
		TargetQID:     opts.TargetQID,
		RecursiveP131: opts.RecursiveP131,
		Languages:     opts.Languages,
		OutItemsPath:  opts.OutItemsPath,
		OutPropsPath:  opts.OutPropsPath,
		Overwrite:     opts.Overwrite,
	}
}

// toEtlProgress turns the progress of the pipeline into the view.
func toEtlProgress(progress etlpipeline.Progress) EtlProgress {
	return EtlProgress{
		ID:                  progress.ID,
		Status:              progress.Status,
		Command:             progress.Command,
		ProcessedLines:      progress.ProcessedLines,
		MatchedItems:        progress.MatchedItems,
		CollectedProperties: progress.CollectedProperties,
		SpeedMBs:            progress.SpeedMBs,
		StartedAt:           formatEtlTime(progress.StartedAt),
		FinishedAt:          formatEtlTime(progress.FinishedAt),
		Error:               progress.Error,
		ItemFile:            progress.ItemFile,
		PropFile:            progress.PropFile,
	}
}

// toEtlHistoryEntry turns an entry of the history into the view.
func toEtlHistoryEntry(entry etlhistory.Entry) EtlHistoryEntry {
	return EtlHistoryEntry{
		ID:            entry.ID,
		FullCommand:   entry.FullCommand,
		TargetQID:     entry.TargetQID,
		RecursiveP131: entry.RecursiveP131,
		Langs:         entry.Langs,
		DumpFilePath:  entry.DumpFilePath,
		OutItemsPath:  entry.OutItemsPath,
		OutPropsPath:  entry.OutPropsPath,
		StartedAt:     entry.StartedAt,
		FinishedAt:    entry.FinishedAt,
		Status:        entry.Status,
		ErrorMessage:  entry.ErrorMessage,
		Notes:         entry.Notes,
	}
}

// formatEtlTime writes a time for the interface.
//
// The time is written in the local zone and to the second: the
// history is read by a person.
func formatEtlTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}

	return value.Local().Format("2006-01-02 15:04:05")
}

// BuildEtlCommand returns the shell pipeline that runs the given job.
//
// The command is the whole pipeline: the decompressor and the worker
// of this same binary. It is what the interface shows, and what the
// user copies to run the job in a terminal.
//
// The command is built without running anything, so the user can see
// it before the job starts.
func (s *AppService) BuildEtlCommand(opts EtlJobOptions) (string, error) {
	if s.etl == nil {
		return "", fmt.Errorf("etlpipeline: the pipeline is not available")
	}
	return s.etl.Command(toEtlJobOptions(opts))
}

// StartEtlJob runs a job.
//
// The pipeline is started in the background, and the progress is pushed
// to the frontend as a Wails event. The run is recorded in the history,
// and the output is imported when the run succeeds.
func (s *AppService) StartEtlJob(opts EtlJobOptions) (EtlProgress, error) {
	if s.etl == nil {
		return EtlProgress{}, fmt.Errorf("etlpipeline: the pipeline is not available")
	}
	progress, err := s.etl.Start(context.Background(), toEtlJobOptions(opts))
	if err != nil {
		return EtlProgress{}, err
	}
	return toEtlProgress(progress), nil
}

// CancelEtlJob stops a running job.
//
// The whole process tree is stopped: the decompressor and the worker
// die together, and nothing is left running in the background.
func (s *AppService) CancelEtlJob(runID string) error {
	if s.etl == nil {
		return fmt.Errorf("etlpipeline: the pipeline is not available")
	}
	return s.etl.Cancel(runID)
}

// ListEtlHistory returns the runs of the extraction worker.
//
// The most recent run is the first one.
func (s *AppService) ListEtlHistory(limit int) ([]EtlHistoryEntry, error) {
	if s.etl == nil {
		return nil, fmt.Errorf("etlpipeline: the pipeline is not available")
	}
	entries, err := s.etl.List(limit)
	if err != nil {
		return nil, err
	}

	views := make([]EtlHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		views = append(views, toEtlHistoryEntry(entry))
	}

	return views, nil
}

// DeleteEtlHistoryEntry removes a run from the history.
func (s *AppService) DeleteEtlHistoryEntry(recordID string) error {
	if s.etl == nil {
		return fmt.Errorf("etlpipeline: the pipeline is not available")
	}
	return s.etl.Delete(recordID)
}

// labelsOf turns the terms of a record into plain text, by language.
func labelsOf(terms map[string]wikidata.Term) map[string]string {
	out := make(map[string]string, len(terms))
	for language, term := range terms {
		out[language] = term.Value
	}
	return out
}

// claimsOf turns the statements of a record into plain values.
//
// The statements are raw JSON, and the frontend reads them as such.
func claimsOf(claims map[string]json.RawMessage) map[string]any {
	out := make(map[string]any, len(claims))
	for property, raw := range claims {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			// A statement that does not decode is skipped: the
			// rest of the record is still worth showing.
			continue
		}
		out[property] = value
	}
	return out
}

// AppName is the name of the application.
const AppName = "WikiRevele"

// AppVersion is the version of the application.
const AppVersion = "0.1.0"
