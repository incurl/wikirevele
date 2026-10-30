package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/incurl/wikirevele/internal/console"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/filesystem"
)

// EventEmitter is notified whenever a job changes state. The Wails
// layer implements it to push updates to the frontend.
type EventEmitter interface {
	// EmitJobUpdated publishes an updated job.
	EmitJobUpdated(job Job)
}

// Manager orchestrates jobs.
//
// The manager owns the lifecycle only: persistence, scheduling,
// cancellation and the status transitions. The work is delegated to the
// Runner.
type Manager struct {
	app     core.App
	emitter EventEmitter
	runner  Runner

	// runnerFn lazily resolves the runner, if one was configured.
	runnerFn func() Runner

	// log records the job traffic for the debug console. It may be nil.
	log *console.Log

	// mu guards the cancellation map.
	mu sync.Mutex
	// cancels holds the cancel function of each running job.
	cancels map[string]context.CancelFunc

	// queue bounds how many jobs run at once.
	queue chan struct{}

	// rootCtx is the context of the whole manager. It is cancelled by
	// Shutdown, which stops the running jobs.
	rootCtx context.Context
	// rootCancel cancels rootCtx.
	rootCancel context.CancelFunc
	// wg tracks the running jobs, so that Shutdown can wait for them.
	wg sync.WaitGroup

	// timeout bounds how long a single job may run. Zero means no
	// bound.
	timeout time.Duration
}

// DefaultTimeout is how long a single job may run.
//
// It is a safety net: a runner that never returns would otherwise hold
// its slot forever and the application would stop making progress.
//
// An application whose work legitimately takes longer raises it with
// WithTimeout, or sets it to zero to remove the bound.
const DefaultTimeout = 30 * time.Minute

// ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithRunner sets the runner that executes the jobs.
func WithRunner(r Runner) ManagerOption {
	return func(m *Manager) { m.runner = r }
}

// WithRunnerFunc sets a lazily resolved runner. It is used when the
// runner depends on the settings, which are read after the manager is
// created.
func WithRunnerFunc(fn func() Runner) ManagerOption {
	return func(m *Manager) { m.runnerFn = fn }
}

// WithLog attaches the debug console log, so that the job traffic is
// recorded for the console panel.
func WithLog(l *console.Log) ManagerOption {
	return func(m *Manager) { m.log = l }
}

// WithConcurrency sets the number of jobs executed in parallel.
func WithConcurrency(n int) ManagerOption {
	return func(m *Manager) {
		if n <= 0 {
			n = 1
		}
		m.queue = make(chan struct{}, n)
	}
}

// WithTimeout sets how long a single job may run.
//
// A job that exceeds the timeout is cancelled and marked as failed.
// Without it, a runner that never returns holds its slot forever and
// the application stops making progress.
//
// Zero means no bound.
func WithTimeout(d time.Duration) ManagerOption {
	return func(m *Manager) { m.timeout = d }
}

// NewManager creates a job manager.
func NewManager(app core.App, opts ...ManagerOption) *Manager {
	m := &Manager{
		app:     app,
		cancels: make(map[string]context.CancelFunc),
		queue:   make(chan struct{}, 2),
	}
	m.rootCtx, m.rootCancel = context.WithCancel(context.Background())
	m.timeout = DefaultTimeout

	for _, opt := range opts {
		if opt != nil {
			opt(m)
		}
	}
	return m
}

// SetEmitter attaches the job update emitter.
//
// The emitter needs the Wails application, which is created after the
// manager, so it is attached once the application exists rather than
// passed to the constructor.
func (m *Manager) SetEmitter(e EventEmitter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.emitter = e
}

// currentEmitter returns the active emitter, or nil.
func (m *Manager) currentEmitter() EventEmitter {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.emitter
}

// currentRunner resolves the active runner.
func (m *Manager) currentRunner() Runner {
	m.mu.Lock()
	runner := m.runner
	fn := m.runnerFn
	m.mu.Unlock()

	if runner == nil && fn != nil {
		runner = fn()
	}
	return runner
}

// Submit creates a job record and schedules it for execution.
//
// The job is persisted with status "pending" before it is queued, so the
// UI can display it immediately.
//
// It takes no context, because it does not wait for anything: the job
// runs on its own goroutine under the manager's context. Cancelling a
// submitted job is Cancel's job.
func (m *Manager) Submit(req Request) (Job, error) {
	if req.Prompt == "" {
		return Job{}, fmt.Errorf("a prompt is required")
	}

	col, err := m.app.FindCollectionByNameOrId(CollectionJobs)
	if err != nil {
		return Job{}, fmt.Errorf("find jobs collection: %w", err)
	}

	title := req.Title
	if title == "" {
		title = defaultTitle(req.Prompt)
	}

	record := core.NewRecord(col)
	record.Set("title", title)
	record.Set("prompt", req.Prompt)
	record.Set("status", StatusPending)
	if len(req.Params) > 0 {
		record.Set("params", req.Params)
	} else {
		record.Set("params", map[string]any{})
	}

	if err := m.app.Save(record); err != nil {
		return Job{}, fmt.Errorf("create job: %w", err)
	}

	job, err := m.toJob(record)
	if err != nil {
		return Job{}, err
	}

	// Link the uploaded inputs to the job now that it has an id.
	if err := m.linkArtifacts(record.Id, req.InputArtifactIDs); err != nil {
		return Job{}, err
	}

	go m.run(record.Id)

	return job, nil
}

// defaultTitle derives a short label from the prompt.
//
// It is the first line of the prompt, truncated, so the queue shows
// something meaningful before the application sets its own title.
func defaultTitle(prompt string) string {
	const max = 60
	title := prompt
	if idx := indexNewline(title); idx >= 0 {
		title = title[:idx]
	}
	title = trimSpace(title)
	if len(title) > max {
		// The cut must not split a rune in half: the byte at the cut
		// position has to be the start of a rune, so that the text
		// before it ends on a rune boundary.
		cut := max
		for cut > 0 && !isRuneStart(title[cut]) {
			cut--
		}
		title = title[:cut] + "…"
	}
	if title == "" {
		return "Job"
	}
	return title
}

// linkArtifacts binds the input artifacts to the job.
//
// An artifact that already belongs to another job is **copied** rather
// than moved: the original job keeps it.
//
// Moving it would be a data-loss bug. A retry re-uses the inputs of the
// job it repeats, and moving them would strip the original job of its
// outputs and hand them to the retry. Deleting the retry would then
// delete files the original still refers to.
//
// An artifact that belongs to no job is simply claimed.
func (m *Manager) linkArtifacts(jobID string, artifactIDs []string) error {
	for _, artifactID := range artifactIDs {
		if artifactID == "" {
			continue
		}
		record, err := m.app.FindRecordById(CollectionArtifacts, artifactID)
		if err != nil {
			return fmt.Errorf("find artifact %q: %w", artifactID, err)
		}

		owner := record.GetString("job")
		if owner == jobID {
			continue
		}

		if owner == "" {
			// The artifact is unowned: claim it.
			record.Set("job", jobID)
			if err := m.app.Save(record); err != nil {
				return fmt.Errorf("link artifact %q: %w", artifactID, err)
			}
			continue
		}

		// The artifact belongs to another job: copy it, so that both
		// jobs keep their own.
		if _, err := m.copyArtifact(record, jobID); err != nil {
			return fmt.Errorf("copy artifact %q: %w", artifactID, err)
		}
	}
	return nil
}

// copyArtifact creates a new artifact record that holds the same bytes
// as the given one and belongs to jobID.
//
// The file itself is not duplicated on disk: PocketBase stores the file
// under a name derived from the record, so the copy points at its own
// stored file. The bytes are read once and written again, which is
// cheap compared with the alternative of sharing a file across records.
func (m *Manager) copyArtifact(record *core.Record, jobID string) (*core.Record, error) {
	data, err := readFileBytes(m.app, record)
	if err != nil {
		return nil, err
	}

	col, err := m.app.FindCollectionByNameOrId(CollectionArtifacts)
	if err != nil {
		return nil, fmt.Errorf("find artifacts collection: %w", err)
	}

	filename := record.GetString("name")
	if filename == "" {
		filename = record.GetString("file")
	}

	file, err := filesystem.NewFileFromBytes(data, filename)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}

	copied := core.NewRecord(col)
	copied.Set("job", jobID)
	copied.Set("kind", record.GetString("kind"))
	copied.Set("file", file)
	copied.Set("name", filename)
	copied.Set("mime", record.GetString("mime"))
	copied.Set("bytes", record.GetInt("bytes"))
	if width := record.GetInt("width"); width != 0 {
		copied.Set("width", width)
	}
	if height := record.GetInt("height"); height != 0 {
		copied.Set("height", height)
	}
	if meta := record.Get("meta"); meta != nil {
		copied.Set("meta", meta)
	}

	if err := m.app.Save(copied); err != nil {
		return nil, fmt.Errorf("save artifact copy: %w", err)
	}
	return copied, nil
}

// Cancel aborts a pending or running job.
func (m *Manager) Cancel(jobID string) error {
	m.mu.Lock()
	cancel, running := m.cancels[jobID]
	m.mu.Unlock()

	if running && cancel != nil {
		cancel()
	}

	record, err := m.app.FindRecordById(CollectionJobs, jobID)
	if err != nil {
		return fmt.Errorf("find job %q: %w", jobID, err)
	}

	status := record.GetString("status")
	if status == StatusCompleted || status == StatusCancelled {
		return nil
	}
	if status == StatusFailed {
		return fmt.Errorf("job %q already failed and cannot be cancelled", jobID)
	}

	record.Set("status", StatusCancelled)
	if err := m.app.Save(record); err != nil {
		return fmt.Errorf("cancel job %q: %w", jobID, err)
	}

	m.emitRecord(record)
	return nil
}

// DeleteJob removes a finished job from the queue.
//
// The artifacts the job produced are removed with it.
func (m *Manager) DeleteJob(jobID string) error {
	record, err := m.app.FindRecordById(CollectionJobs, jobID)
	if err != nil {
		return fmt.Errorf("find job %q: %w", jobID, err)
	}

	status := record.GetString("status")
	if status == StatusPending || status == StatusRunning {
		return fmt.Errorf("job %q is still running and cannot be deleted", jobID)
	}

	// Remove the artifacts the job produced, so that no orphan files
	// are left in the storage directory.
	artifacts, err := m.app.FindRecordsByFilter(
		CollectionArtifacts,
		"job = {:job}",
		"",
		0,
		0,
		dbx.Params{"job": jobID},
	)
	if err == nil {
		for _, artifact := range artifacts {
			if err := m.app.Delete(artifact); err != nil {
				return fmt.Errorf("delete artifact %q: %w", artifact.Id, err)
			}
		}
	}

	if err := m.app.Delete(record); err != nil {
		return fmt.Errorf("delete job %q: %w", jobID, err)
	}
	return nil
}

// ListJobs returns the recorded jobs, newest first.
func (m *Manager) ListJobs(limit int) ([]Job, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	records, err := m.app.FindRecordsByFilter(
		CollectionJobs,
		"",
		"-created",
		limit,
		0,
	)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}

	// The jobs of the whole page are converted at once, so that the
	// artifacts are fetched in a single query instead of one per job.
	return m.toJobs(records)
}

// GetJob returns a single job.
func (m *Manager) GetJob(jobID string) (Job, error) {
	record, err := m.app.FindRecordById(CollectionJobs, jobID)
	if err != nil {
		return Job{}, fmt.Errorf("find job %q: %w", jobID, err)
	}
	return m.toJob(record)
}

// jobContext builds the context a single job runs under.
//
// Exactly one context is created, so that its cancel function is the
// one the caller holds. Building a cancellable context and then
// replacing it with a timeout would drop the first cancel function and
// leak the first context for the life of the manager.
func (m *Manager) jobContext() (context.Context, context.CancelFunc) {
	if m.timeout > 0 {
		return context.WithTimeout(m.rootCtx, m.timeout)
	}
	return context.WithCancel(m.rootCtx)
}

// run executes a job.
//
// It acquires a slot in the concurrency queue, transitions the job to
// running, calls the runner and records the outcome.
func (m *Manager) run(jobID string) {
	// Wait for a free slot. The queue bounds the parallelism so that a
	// burst of submissions cannot flood a rate-limited backend.
	m.queue <- struct{}{}
	defer func() { <-m.queue }()

	record, err := m.app.FindRecordById(CollectionJobs, jobID)
	if err != nil {
		m.logError(jobID, fmt.Errorf("find job: %w", err))
		return
	}

	// The job may have been cancelled while it was waiting for a slot.
	if record.GetString("status") == StatusCancelled {
		return
	}

	runner := m.currentRunner()
	if runner == nil {
		m.failJob(jobID, 0, "no runner is registered. Register one before submitting jobs.")
		return
	}

	// The job context derives from the manager's root context, so that
	// a shutdown stops it, and it carries the timeout, so that a runner
	// that never returns cannot hold its slot forever.
	ctx, cancel := m.jobContext()

	m.mu.Lock()
	m.cancels[jobID] = cancel
	m.mu.Unlock()

	// The job is tracked, so that Shutdown can wait for it.
	m.wg.Add(1)
	defer func() {
		cancel()
		m.mu.Lock()
		delete(m.cancels, jobID)
		m.mu.Unlock()
		m.wg.Done()
	}()

	record.Set("status", StatusRunning)
	if err := m.app.Save(record); err != nil {
		m.logError(jobID, fmt.Errorf("mark running: %w", err))
		return
	}
	m.emitRecord(record)

	job, err := m.toJob(record)
	if err != nil {
		m.logError(jobID, err)
		return
	}

	m.logInfo(jobID, "the job started")

	result, runErr := runner.Run(ctx, job)
	if runErr != nil {
		// A job that was cancelled by the user keeps its cancelled
		// status: the user asked for it, it is not a failure.
		if errors.Is(ctx.Err(), context.Canceled) && m.rootCtx.Err() == nil {
			m.logInfo(jobID, "the job was cancelled")
			return
		}

		// A job that ran out of time is a failure: the runner did not
		// finish in the time it was given.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			m.failJob(jobID, 0, fmt.Sprintf(
				"The job exceeded the time limit of %s and was stopped.", m.timeout))
			return
		}

		// A job that was interrupted by the shutdown is marked as
		// failed, because it will never be resumed.
		if m.rootCtx.Err() != nil {
			m.failJob(jobID, 0, "The application was closed while the job was running.")
			return
		}

		code, message := describeError(runErr)
		m.failJob(jobID, code, message)
		return
	}

	m.completeJob(jobID, result)
}

// completeJob marks a job as completed and stores its result.
func (m *Manager) completeJob(jobID string, result *Result) {
	record, err := m.app.FindRecordById(CollectionJobs, jobID)
	if err != nil {
		m.logError(jobID, fmt.Errorf("find job: %w", err))
		return
	}

	record.Set("status", StatusCompleted)
	if result != nil && len(result.Payload) > 0 {
		record.Set("result", result.Payload)
	}

	if err := m.app.Save(record); err != nil {
		m.logError(jobID, fmt.Errorf("mark completed: %w", err))
		return
	}

	m.emitRecord(record)
	m.logInfo(jobID, "the job completed")
}

// failJob marks a job as failed and records the error.
func (m *Manager) failJob(jobID string, code int, message string) {
	record, err := m.app.FindRecordById(CollectionJobs, jobID)
	if err != nil {
		m.logError(jobID, fmt.Errorf("find job: %w", err))
		return
	}

	record.Set("status", StatusFailed)
	if code != 0 {
		record.Set("error_code", code)
	}
	record.Set("error_message", message)

	if err := m.app.Save(record); err != nil {
		m.logError(jobID, fmt.Errorf("mark failed: %w", err))
		return
	}

	m.emitRecord(record)
	m.logError(jobID, fmt.Errorf("%s", message))
}

// describeError extracts the code and the message from an error.
//
// A Runner may return a *Error to attach a numeric code; anything else
// is reported with its own text and no code.
//
// errors.As is used rather than a hand-rolled unwrap loop, because it
// also walks errors.Join's Unwrap() []error. A runner that reports the
// API failure and a cleanup failure together would otherwise lose the
// code.
func describeError(err error) (int, string) {
	var coded *Error
	if errors.As(err, &coded) {
		return coded.Code, coded.Error()
	}
	return 0, err.Error()
}

// logInfo records a normal event for the debug console.
func (m *Manager) logInfo(jobID, message string) {
	if m.log == nil {
		return
	}
	m.log.Info("job "+jobID, message)
}

// logError records a failure for the debug console.
func (m *Manager) logError(jobID string, cause error) {
	if m.log == nil {
		return
	}
	m.log.Error("job "+jobID, cause.Error())
}

// emitRecord publishes a job update.
func (m *Manager) emitRecord(record *core.Record) {
	emitter := m.currentEmitter()
	if emitter == nil {
		return
	}
	job, err := m.toJob(record)
	if err != nil {
		return
	}
	emitter.EmitJobUpdated(job)
}

// toJob converts a PocketBase record into the frontend-facing job.
//
// It collects the artifacts of the job with a query of its own. Use
// toJobs when several jobs are converted at once: it fetches the
// artifacts of the whole page in a single query.
func (m *Manager) toJob(record *core.Record) (Job, error) {
	job := recordToJob(record)

	artifacts, err := m.artifactsOfJobs([]string{record.Id})
	if err != nil {
		return Job{}, err
	}
	job.ArtifactIDs, job.InputArtifactIDs = splitArtifacts(artifacts[record.Id])

	return job, nil
}

// toJobs converts several PocketBase records into the frontend-facing
// jobs, fetching the artifacts of the whole page in a single query.
//
// Without it, a page of N jobs costs N+1 queries, because each job
// collects its artifacts on its own. The frontend polls this every five
// seconds, so the difference grows with the history.
func (m *Manager) toJobs(records []*core.Record) ([]Job, error) {
	if len(records) == 0 {
		return []Job{}, nil
	}

	jobIDs := make([]string, 0, len(records))
	for _, record := range records {
		jobIDs = append(jobIDs, record.Id)
	}

	// One query for the artifacts of the whole page. A failure here is
	// not fatal: the jobs are still listed, just without their
	// artifacts.
	artifacts, artifactsErr := m.artifactsOfJobs(jobIDs)

	jobs := make([]Job, 0, len(records))
	for _, record := range records {
		job := recordToJob(record)
		if artifactsErr == nil {
			job.ArtifactIDs, job.InputArtifactIDs = splitArtifacts(artifacts[record.Id])
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// artifactsOfJobs fetches the artifacts of several jobs in one query.
//
// The artifacts are returned per job id, in the order they were
// created, newest first.
func (m *Manager) artifactsOfJobs(jobIDs []string) (map[string][]*core.Record, error) {
	byJob := map[string][]*core.Record{}
	if len(jobIDs) == 0 {
		return byJob, nil
	}

	values := make([]any, 0, len(jobIDs))
	for _, id := range jobIDs {
		values = append(values, id)
	}

	// The ORM query is used, rather than a raw one, so that the
	// records are scanned with their collection context. A raw query
	// panics in PostScan, because the record has no collection.
	records := []*core.Record{}
	err := m.app.RecordQuery(CollectionArtifacts).
		Where(dbx.In("job", values...)).
		OrderBy("created DESC").
		All(&records)
	if err != nil {
		return nil, fmt.Errorf("find the artifacts: %w", err)
	}

	for _, record := range records {
		jobID := record.GetString("job")
		byJob[jobID] = append(byJob[jobID], record)
	}
	return byJob, nil
}

// splitArtifacts divides the artifacts of a job into its outputs and
// its inputs.
//
// The inputs and the outputs are kept apart, because a retry re-uses
// the inputs while the outputs stay with the job that produced them.
func splitArtifacts(records []*core.Record) (outputs []string, inputs []string) {
	outputs = []string{}
	inputs = []string{}

	for _, record := range records {
		if record.GetString("kind") == ArtifactKindInput {
			inputs = append(inputs, record.Id)
			continue
		}
		outputs = append(outputs, record.Id)
	}
	return outputs, inputs
}

// recordToJob converts a PocketBase record into the frontend-facing
// job, without collecting its artifacts.
func recordToJob(record *core.Record) Job {
	job := Job{
		ID:               record.Id,
		Title:            record.GetString("title"),
		Prompt:           record.GetString("prompt"),
		Params:           map[string]any{},
		Result:           map[string]any{},
		Status:           record.GetString("status"),
		ErrorMessage:     record.GetString("error_message"),
		ArtifactIDs:      []string{},
		InputArtifactIDs: []string{},
		Created:          record.GetString("created"),
		Updated:          record.GetString("updated"),
	}

	if raw := record.Get("params"); raw != nil {
		if encoded, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(encoded, &job.Params)
		}
	}
	if job.Params == nil {
		job.Params = map[string]any{}
	}

	if raw := record.Get("result"); raw != nil {
		if encoded, err := json.Marshal(raw); err == nil {
			_ = json.Unmarshal(encoded, &job.Result)
		}
	}
	if job.Result == nil {
		job.Result = map[string]any{}
	}

	if code := record.GetInt("error_code"); code != 0 {
		job.ErrorCode = code
	}

	return job
}

// StoreArtifact saves a file produced by a job.
//
// The file is stored using PocketBase's native file field, so it is
// served by the built-in file route and inherits the collection's access
// rules.
func (m *Manager) StoreArtifact(jobID, kind string, data []byte, filename string, meta map[string]any) (Artifact, error) {
	if kind != ArtifactKindInput && kind != ArtifactKindOutput {
		return Artifact{}, fmt.Errorf("unknown artifact kind %q", kind)
	}
	if len(data) == 0 {
		return Artifact{}, fmt.Errorf("the file is empty")
	}

	col, err := m.app.FindCollectionByNameOrId(CollectionArtifacts)
	if err != nil {
		return Artifact{}, fmt.Errorf("find artifacts collection: %w", err)
	}

	if filename == "" {
		filename = "artifact." + DetectMediaType(data).Extension
	}

	file, err := filesystem.NewFileFromBytes(data, filename)
	if err != nil {
		return Artifact{}, fmt.Errorf("create file: %w", err)
	}

	record := core.NewRecord(col)
	if jobID != "" {
		record.Set("job", jobID)
	}
	record.Set("kind", kind)
	record.Set("file", file)
	record.Set("name", filename)
	record.Set("mime", MimeTypeOf(data))
	record.Set("bytes", len(data))

	if size, dimErr := DimensionsOf(data); dimErr == nil {
		record.Set("width", size.Width)
		record.Set("height", size.Height)
	}
	if len(meta) > 0 {
		record.Set("meta", meta)
	}

	if err := m.app.Save(record); err != nil {
		return Artifact{}, fmt.Errorf("save artifact: %w", err)
	}

	return m.toArtifact(record), nil
}
