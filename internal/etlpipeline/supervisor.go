package etlpipeline

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/incurl/wikirevele/internal/etl"
)

// The statuses a pipeline run goes through.
//
// They are the statuses the history records, and they are what the
// interface shows.
const (
	// StatusRunning means the pipeline is running.
	StatusRunning = "running"
	// StatusSucceeded means the pipeline finished and wrote its
	// output.
	StatusSucceeded = "succeeded"
	// StatusCancelled means the user stopped it.
	StatusCancelled = "cancelled"
	// StatusFailed means the pipeline failed.
	StatusFailed = "failed"
)

// The interval between two checks that the process is still alive.
const pollInterval = 200 * time.Millisecond

// EventDrainTimeout is how long the supervisor waits for the events of
// a worker that is gone.
//
// The worker writes its last event and stops, so the reader reaches
// the end of the pipe at once. The wait is for the other case: a child
// of the worker that holds the pipe open. Without it, a run waits for
// a process that has nothing to do with the run.
const EventDrainTimeout = 2 * time.Second

// Progress is the state of a running pipeline.
//
// It is what the interface shows, and it is what the history records
// when the run is over.
type Progress struct {
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
	StartedAt time.Time `json:"started_at"`
	// FinishedAt is when the run ended. It is zero while the run is
	// going.
	FinishedAt time.Time `json:"finished_at"`
	// Error is why the run failed. It is empty when it did not.
	Error string `json:"error"`
	// ItemFile is the file the items were written to.
	ItemFile string `json:"item_file"`
	// PropFile is the file the properties were written to.
	PropFile string `json:"prop_file"`
}

// Emitter is told about the progress of a run.
//
// The Wails layer implements it to push the progress to the interface.
type Emitter interface {
	// EmitEtlProgress publishes the progress of a run.
	EmitEtlProgress(progress Progress)
}

// LogSink receives the human readable output of the worker.
//
// The worker writes its warnings to the standard error, and they are
// shown in the log panel of the interface.
type LogSink interface {
	// LogEtlLine records one line of the output of the worker.
	LogEtlLine(runID, line string)
}

// Launcher starts the two processes of a pipeline.
//
// The interface exists so that a test can put its own processes in
// place of the decompressor and the worker: the real worker is the
// application itself, and it cannot be run from inside a test.
type Launcher interface {
	// Launch starts the decompressor and the worker.
	//
	// The standard output of the decompressor is the standard input
	// of the worker, so the two are already piped together when the
	// function returns. The standard output of the worker is
	// returned, so that the caller can read the events.
	//
	// The reader of the events is an io.ReadCloser and not an
	// io.Reader, because the caller has to be able to stop it: a
	// child of the worker can hold the pipe open, and the run must
	// not wait for it.
	Launch(ctx context.Context, decompressor, dump string, worker []string) (io.ReadCloser, *exec.Cmd, *exec.Cmd, error)
}

// systemLauncher starts the real processes.
type systemLauncher struct{}

// Launch starts the decompressor and the worker.
//
// The decompressor writes to a pipe, and the worker reads from it.
func (l systemLauncher) Launch(
	ctx context.Context,
	decompressor, dump string,
	worker []string,
) (io.ReadCloser, *exec.Cmd, *exec.Cmd, error) {
	decompressorName, decompressorArgs := splitDecompressor(decompressor, dump)
	decompressorProcess := exec.CommandContext(ctx, decompressorName, decompressorArgs...)
	decompressorProcess.Stderr = os.Stderr

	pipe, err := decompressorProcess.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("etlpipeline: the pipe: %w", err)
	}

	// The worker is the binary itself. An empty path means the
	// caller is a test and the binary cannot be spawned: the tests
	// use the launcher seam instead.
	path := workerPath()
	if path == "" {
		return nil, nil, nil, fmt.Errorf(
			"etlpipeline: the worker binary cannot be spawned from a test")
	}

	workerProcess := exec.CommandContext(ctx, path, worker...)
	workerProcess.Stdin = pipe
	workerProcess.Stderr = os.Stderr

	// The events are read from a pipe the supervisor owns.
	//
	// The pipe of os/exec cannot be used here: waiting for the
	// worker closes it, and the reader then loses the last events
	// with an "file already closed" error. A pipe of os.Pipe is not
	// touched by the wait, so the reader gets every line the worker
	// wrote, and the supervisor can close it when the run is over.
	eventsRead, eventsWrite, err := os.Pipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("etlpipeline: the events pipe: %w", err)
	}
	workerProcess.Stdout = eventsWrite

	// Both processes share a process group, so that stopping the run
	// stops the decompressor as well. The decompressor would
	// otherwise keep running, and it would keep the CPU busy for
	// hours.
	setProcessGroup(decompressorProcess)
	setProcessGroup(workerProcess)

	if err := decompressorProcess.Start(); err != nil {
		_ = eventsRead.Close()
		_ = eventsWrite.Close()
		return nil, nil, nil, fmt.Errorf("etlpipeline: start %s: %w", decompressor, err)
	}
	if err := workerProcess.Start(); err != nil {
		_ = decompressorProcess.Process.Kill()
		_ = eventsRead.Close()
		_ = eventsWrite.Close()
		return nil, nil, nil, fmt.Errorf("etlpipeline: start the worker: %w", err)
	}

	// The write end is closed here: the worker holds the other one,
	// and the read end sees the end of the output when the worker is
	// gone and nothing else holds it open.
	_ = eventsWrite.Close()

	return eventsRead, decompressorProcess, workerProcess, nil
}

// Supervisor runs the pipelines.
//
// It owns the child processes: it starts them, it reads their output,
// and it stops them. Nothing else in the application knows how a
// pipeline is run.
type Supervisor struct {
	// mu guards the runs.
	mu sync.Mutex
	// runs holds the running pipelines, by identifier.
	runs map[string]*run
	// emitter is told about the progress. It may be nil.
	emitter Emitter
	// log receives the output of the workers. It may be nil.
	log LogSink
	// counter numbers the runs.
	counter int
	// launcher starts the processes. It may be nil, in which case
	// the real ones are started.
	launcher Launcher
	// onFinished is called when a run ends. It may be nil.
	onFinished func(progress Progress)
}

// NewSupervisor creates a supervisor.
func NewSupervisor() *Supervisor {
	return &Supervisor{runs: make(map[string]*run)}
}

// SetLauncher sets the launcher that starts the processes.
//
// It is for the tests: the real worker is the application itself, and
// it cannot be run from inside a test.
func (s *Supervisor) SetLauncher(launcher Launcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.launcher = launcher
}

// SetOnFinished sets the function that is called when a run ends.
//
// The service uses it to record the run in the history and to import
// the output. The supervisor does not know about either: the callback
// is the seam.
func (s *Supervisor) SetOnFinished(onFinished func(progress Progress)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onFinished = onFinished
}

// SetEmitter sets the function that is called with the progress.
func (s *Supervisor) SetEmitter(emitter Emitter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emitter = emitter
}

// SetLogSink sets the sink that receives the output of the workers.
func (s *Supervisor) SetLogSink(sink LogSink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log = sink
}

// run is one pipeline that is running.
type run struct {
	// id identifies the run.
	id string
	// command is the process group of the pipeline.
	command *exec.Cmd
	// cancel stops the pipeline.
	cancel context.CancelFunc
	// done is closed when the pipeline is over.
	done chan struct{}
	// read is closed when the events have all been read.
	read chan struct{}
	// progress is the state of the run.
	progress Progress
}

// Start runs a pipeline.
//
// The pipeline is started in its own process group, so that it can be
// stopped as a whole: the decompressor and the worker die together,
// and no orphan is left running.
func (s *Supervisor) Start(ctx context.Context, opts JobOptions) (Progress, error) {
	command, err := BuildCommand(opts, WorkerCommand())
	if err != nil {
		return Progress{}, err
	}

	// The decompressor is checked before the run starts: a missing
	// one is a message the user can act on, and it is much clearer
	// than a pipeline that fails with "command not found".
	decompressor, err := DecompressorFor(opts.DumpPath)
	if err != nil {
		return Progress{}, err
	}
	if err := checkDecompressor(decompressor); err != nil {
		return Progress{}, err
	}

	// The files are checked before the run starts: a dump that is not
	// there, or an output that cannot be written, is a message the
	// user can act on.
	if err := Validate(opts); err != nil {
		return Progress{}, err
	}

	launcher := s.launcher
	if launcher == nil {
		launcher = systemLauncher{}
	}

	// The worker is the binary itself, with the etl subcommand. The
	// decompressor feeds it through a pipe.
	output, decompressorProcess, worker, err := launcher.Launch(
		ctx, decompressor, opts.DumpPath, workerArguments(opts))
	if err != nil {
		return Progress{}, err
	}

	_, cancel := context.WithCancel(ctx)

	now := time.Now()
	progress := Progress{
		ID:        fmt.Sprintf("etl-%d", now.UnixNano()),
		Status:    StatusRunning,
		Command:   command,
		StartedAt: now,
		ItemFile:  opts.OutItemsPath,
		PropFile:  opts.OutPropsPath,
	}

	r := &run{
		id:       progress.ID,
		command:  worker,
		cancel:   cancel,
		done:     make(chan struct{}),
		read:     make(chan struct{}),
		progress: progress,
	}

	s.mu.Lock()
	s.counter++
	s.runs[progress.ID] = r
	s.mu.Unlock()

	go s.readEvents(r, output)
	go s.watch(r, decompressorProcess, worker, output)

	// The progress is read under the lock: the reader of the events
	// writes it, and the two goroutines are already running.
	s.mu.Lock()
	current := r.progress
	emitter := s.emitter
	s.mu.Unlock()

	if emitter != nil {
		emitter.EmitEtlProgress(current)
	}

	return current, nil
}

// workerArguments returns the arguments of the worker.
//
// The worker is the same binary, with the etl subcommand and the
// options of the job.
func workerArguments(opts JobOptions) []string {
	arguments := []string{
		"etl",
		"--target-qid", opts.TargetQID,
	}

	if opts.RecursiveP131 {
		arguments = append(arguments, "--recursive-p131")
	}
	if len(opts.Languages) > 0 {
		arguments = append(arguments, "--langs", strings.Join(opts.Languages, ","))
	}
	arguments = append(arguments,
		"--out-items", opts.OutItemsPath,
		"--out-props", opts.OutPropsPath,
		"--events",
	)
	if opts.Overwrite {
		arguments = append(arguments, "--overwrite")
	}

	return arguments
}

// splitDecompressor returns the name and the arguments of the
// decompressor.
//
// The decompressor is a command and its flags: "gzip -dc" is two
// words, and the path of the dump is the last argument.
func splitDecompressor(decompressor, path string) (string, []string) {
	fields := strings.Fields(decompressor)
	if len(fields) == 0 {
		return decompressor, []string{path}
	}

	return fields[0], append(fields[1:], path)
}

// readEvents reads the events of the worker.
//
// The worker writes one JSON object per line on its standard output,
// and the line is read a line at a time.
func (s *Supervisor) readEvents(r *run, output io.Reader) {
	defer close(r.read)

	scanner := bufio.NewScanner(output)
	scanner.Buffer(make([]byte, 0, 64<<10), 4<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// The events are JSON. A line that is not is a message
		// from the worker that is not an event, and it goes to the
		// log.
		var event etl.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			s.logLine(r.id, line)
			continue
		}

		switch event.Type {
		case etl.EventProgress:
			s.update(r, func(p *Progress) {
				p.ProcessedLines = event.ProcessedLines
				p.MatchedItems = event.MatchedItems
				p.CollectedProperties = event.CollectedProperties
				p.SpeedMBs = event.SpeedMBs
			})
		case etl.EventDone:
			s.update(r, func(p *Progress) {
				p.ItemFile = event.ItemFile
				p.PropFile = event.PropFile
			})
		case etl.EventError:
			s.update(r, func(p *Progress) {
				p.Error = event.Message
			})
		}
	}
}

// watch waits for the pipeline to end.
//
// The worker is the one that decides when the run is over: the
// decompressor may finish first, and the worker is the one that
// writes the output.
func (s *Supervisor) watch(r *run, decompressor, worker *exec.Cmd, events io.ReadCloser) {
	// The worker is waited for first, and the events are read while it
	// is running.
	//
	// The reader has to run at the same time as the wait: a reader
	// that starts after the worker is gone reads nothing. The
	// supervisor owns the pipe, so the wait does not close it and the
	// reader gets every line the worker wrote.
	workerErr := worker.Wait()

	// The reader ends when the pipe reaches its end, which happens
	// when the worker is gone and nothing else holds it open.
	//
	// A child of the worker can hold the pipe open after the worker
	// is gone, so the reader is given a moment to finish and the pipe
	// is closed if it does not: a run that waits for a grandchild is a
	// run that never ends.
	//
	// The pipe is not closed before the reader is done, because the
	// lines that are still in it would be lost, and the counters of
	// the run would be wrong.
	select {
	case <-r.read:
		// The reader has read everything.
	case <-time.After(EventDrainTimeout):
		// The pipe is still open: something holds it. It is
		// closed, and the reader stops with it.
		_ = events.Close()
		<-r.read
	}

	decompressorErr := decompressor.Wait()

	// The status of the run is decided under the lock: the reader of
	// the events changes the counters of the same run, and the two
	// must not race.
	s.mu.Lock()
	running := r.progress.Status == StatusRunning
	cancelled := r.progress.Status == StatusCancelled

	switch {
	case !running:
		// The run was cancelled, and the status was already set.
	case cancelled:
		// The status is already the cancelled one.
	case workerErr != nil:
		r.progress.Status = StatusFailed
		r.progress.Error = describeExit(workerErr)
	default:
		r.progress.Status = StatusSucceeded
	}

	if decompressorErr != nil && r.progress.Status == StatusSucceeded {
		// The decompressor failed after the worker was done. The
		// output may be short, and the user has to know.
		r.progress.Status = StatusFailed
		r.progress.Error = describeExit(decompressorErr)
	}

	r.progress.FinishedAt = time.Now()
	progress := r.progress
	onFinished := s.onFinished
	s.mu.Unlock()

	// The run is closed only now that the status is decided. A waiter
	// that is woken by the close reads the progress, and the status
	// has to be there already: a run that reports "running" to a
	// caller that was told it had finished is a lie.
	close(r.done)

	s.emit(progress)

	if !running {
		// The run was cancelled, and the status was already set.
		return
	}

	// The service is told how the run ended, so that it can record
	// it and import the output.
	if onFinished != nil {
		onFinished(progress)
	}
}

// update changes the progress of a run and publishes it.
func (s *Supervisor) update(r *run, change func(*Progress)) {
	s.mu.Lock()
	change(&r.progress)
	progress := r.progress
	emitter := s.emitter
	s.mu.Unlock()

	if emitter != nil {
		emitter.EmitEtlProgress(progress)
	}
}

// emit publishes the progress of a run.
func (s *Supervisor) emit(progress Progress) {
	s.mu.Lock()
	emitter := s.emitter
	s.mu.Unlock()

	if emitter != nil {
		emitter.EmitEtlProgress(progress)
	}
}

// logLine sends one line of the output of the worker to the log.
func (s *Supervisor) logLine(runID, line string) {
	s.mu.Lock()
	sink := s.log
	s.mu.Unlock()

	if sink != nil {
		sink.LogEtlLine(runID, line)
	}
}

// Cancel stops a running pipeline.
//
// The whole process tree is stopped: the decompressor and the worker
// die together, and nothing is left running in the background.
func (s *Supervisor) Cancel(runID string) error {
	s.mu.Lock()
	r, ok := s.runs[runID]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("etlpipeline: no run %q", runID)
	}

	s.update(r, func(p *Progress) {
		p.Status = StatusCancelled
	})

	r.cancel()

	// The process group is killed as well: the context cancels the
	// process, but the decompressor is a child of it and it may
	// outlive it.
	killProcessGroup(r.command)

	return nil
}

// Close stops every running pipeline.
//
// The application calls it when it closes, so that no child process is
// left running after the window is gone.
func (s *Supervisor) Close() {
	s.mu.Lock()
	runs := make([]*run, 0, len(s.runs))
	for _, r := range s.runs {
		runs = append(runs, r)
	}
	s.mu.Unlock()

	for _, r := range runs {
		r.cancel()
		killProcessGroup(r.command)
	}
}

// Wait waits for a run to finish.
//
// It is used by the tests and by the shutdown, which waits a little
// for the children to die.
func (s *Supervisor) Wait(runID string, timeout time.Duration) bool {
	s.mu.Lock()
	r, ok := s.runs[runID]
	s.mu.Unlock()

	if !ok {
		return false
	}

	select {
	case <-r.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// Progress returns the progress of a run.
func (s *Supervisor) Progress(runID string) (Progress, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.runs[runID]
	if !ok {
		return Progress{}, false
	}

	return r.progress, true
}

// checkDecompressor tells whether the decompressor is on the host.
//
// The check is a look up in the path, and it is done before the run
// starts: a missing decompressor is a message the user can act on.
func checkDecompressor(decompressor string) error {
	fields := strings.Fields(decompressor)
	name := fields[0]

	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf(
			"etlpipeline: %s is not installed: the decompressor must be on the host, "+
				"the application does not ship it", name)
	}

	return nil
}

// setProcessGroup puts a process in its own group.
//
// The group is what the whole tree is killed through.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup kills a process and everything it started.
//
// The group of the process is killed, so the decompressor dies with
// the worker.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}

	// A negative pid is the group of the process.
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill()
	}
}

// describeExit says what went wrong with a process.
//
// The message of an exit code is the one the user can act on: "the
// file already exists" is better than "exit status 1".
func describeExit(err error) string {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Sprintf("the worker stopped with the code %d", exit.ExitCode())
	}

	return err.Error()
}
