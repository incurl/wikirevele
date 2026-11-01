package sync

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

// Progress is the state of a sync that is running.
//
// It is pushed to the interface as it goes, and it is the only way
// the interface learns that a run is moving.
type Progress struct {
	// Collection is the collection the run is on.
	Collection string `json:"collection"`
	// Direction is the way the records of the collection travel.
	Direction Direction `json:"direction"`
	// Phase is what the run is doing: "schema", "push", "pull" or
	// "done".
	Phase string `json:"phase"`
	// Records is the number of records that were moved.
	Records int `json:"records"`
	// Total is the number of collections of the run.
	Total int `json:"total"`
	// Finished is the number of collections that are finished.
	Finished int `json:"finished"`
	// Running tells whether a run is in progress.
	Running bool `json:"running"`
	// Error is the message of the failure, when there was one.
	Error string `json:"error"`
	// Over tells whether the run is over.
	Over bool `json:"over"`
}

// Emitter is notified whenever the progress of a sync changes.
//
// The Wails layer implements it to push the progress to the frontend.
type Emitter interface {
	// EmitSyncProgress publishes the progress of a sync.
	EmitSyncProgress(progress Progress)
}

// Logger records what a sync did.
//
// The log is written to a file in the data directory of the
// application, because a sync of a million records has more to say
// than the interface can show.
type Logger interface {
	// LogSync records one line of the log of a sync.
	LogSync(message string)
}

// CollectionInfo is one collection that can be synced, as the
// interface sees it.
type CollectionInfo struct {
	// Name is the name of the collection.
	Name string `json:"name"`
	// Type is the type of the collection: "base" or "auth".
	Type string `json:"type"`
	// Records is the number of records it holds.
	Records int64 `json:"records"`
	// HasSyncField tells whether it already carries the fields the
	// sync needs.
	HasSyncField bool `json:"has_sync_field"`
}

// Service runs the syncs.
//
// It owns the set of the collections, the times of the last runs, and
// the runs themselves. The work is done by the client, which talks
// to the remote server, and by the store, which reads and writes the
// local records.
type Service struct {
	// store reads and writes the local records.
	store *LocalStore
	// clientFactory builds the client of the remote server. It is
	// built when a run starts, because the credentials may have
	// changed since the last one.
	clientFactory func() (*Client, error)

	// mu guards the state below.
	mu sync.Mutex
	// progress is the state of the last run.
	progress Progress
	// running tells whether a run is in progress.
	running bool
	// cancel stops the run that is going.
	cancel context.CancelFunc
	// emitter is called with the progress of the run.
	emitter Emitter
	// logger records what the run did.
	logger Logger
	// cursors are the times of the last runs, by collection.
	cursors *CursorSet
	// syncSet is the set of the collections that take part in a sync.
	syncSet SyncSet
}

// NewService creates a service.
//
// The client factory builds the client of the remote server when a
// run starts: the address and the credentials are read from the
// settings then, and not held by the service.
func NewService(store *LocalStore, clientFactory func() (*Client, error)) *Service {
	return &Service{
		store:         store,
		clientFactory: clientFactory,
		progress:      Progress{Over: true},
		cursors:       NewCursorSet(),
	}
}

// SetEmitter attaches the emitter once it becomes available.
func (s *Service) SetEmitter(e Emitter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emitter = e
}

// SetLogger attaches the logger once it becomes available.
func (s *Service) SetLogger(l Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logger = l
}

// SetSyncSet records the set of the collections that take part in a
// sync.
func (s *Service) SetSyncSet(set SyncSet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncSet = set
}

// SyncSet returns the set of the collections that take part in a
// sync.
func (s *Service) SyncSet() SyncSet {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncSet
}

// SetCursors records the times of the last runs.
func (s *Service) SetCursors(cursors *CursorSet) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursors = cursors
}

// Cursors returns the times of the last runs.
func (s *Service) Cursors() *CursorSet {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursors
}

// Progress returns the state of the last run.
func (s *Service) Progress() Progress {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.progress
}

// Running tells whether a run is in progress.
func (s *Service) Running() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Cancel stops the run that is going.
//
// The records that were written before the cancel stay written, and
// the run can be started again: the records are upserted.
func (s *Service) Cancel() {
	s.mu.Lock()
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Collections returns the collections of the local database that can
// be synced.
//
// The system collections are left out: they belong to PocketBase and
// not to the data of the user. The views are left out too: they hold
// no records of their own.
func (s *Service) Collections() ([]CollectionInfo, error) {
	collections, err := s.store.App().FindAllCollections()
	if err != nil {
		return nil, fmt.Errorf("sync: list the collections: %w", err)
	}

	out := make([]CollectionInfo, 0, len(collections))
	for _, collection := range collections {
		if collection.System || collection.IsView() {
			continue
		}

		// A collection that cannot be counted is still offered: the
		// reader may want to sync it and find out why it failed.
		count, err := s.store.App().CountRecords(collection.Name)
		if err != nil {
			count = 0
		}

		out = append(out, CollectionInfo{
			Name:         collection.Name,
			Type:         collection.Type,
			Records:      count,
			HasSyncField: s.store.HasSyncFields(collection),
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Run starts a sync of the whole set.
//
// The run happens in the background, and the progress is pushed to
// the interface as it goes. The method returns as soon as the run is
// under way.
func (s *Service) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("sync: a sync is already running")
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true
	set := s.syncSet
	s.mu.Unlock()

	go func() {
		defer cancel()

		err := s.run(runCtx, set)

		s.mu.Lock()
		s.running = false
		s.cancel = nil
		s.mu.Unlock()

		s.emit(Progress{
			Phase:    phaseOf(err),
			Total:    len(set.Collections),
			Finished: len(set.Collections),
			Running:  false,
			Error:    errorText(err),
			Over:     true,
		})
	}()

	return nil
}

// phaseOf returns the phase of the end of a run.
func phaseOf(err error) string {
	if err != nil {
		return "failed"
	}
	return "done"
}

// errorText returns the message of a failure, or an empty string.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// run does the work of a sync.
func (s *Service) run(ctx context.Context, set SyncSet) error {
	if err := set.Validate(); err != nil {
		return err
	}
	if len(set.Collections) == 0 {
		return fmt.Errorf("sync: no collection was chosen")
	}

	client, err := s.clientFactory()
	if err != nil {
		return err
	}
	if err := client.Configured(); err != nil {
		return err
	}

	// The pre-flight check: the server has to be there and the
	// credentials have to work before anything is touched.
	if err := client.ping(ctx); err != nil {
		return err
	}
	s.logf("the remote server at %s answered", client.BaseURL())

	// The collections are ordered so that a relation always points
	// at something that is already there.
	ordered, err := s.order(set)
	if err != nil {
		return err
	}

	// The fields the sync needs are added to the collections that do
	// not have them, so that a collection of an older database can be
	// synced.
	for _, spec := range ordered {
		if _, err := s.store.EnsureSyncFields(spec.Name); err != nil {
			return err
		}
	}

	total := len(ordered)
	for index, spec := range ordered {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := s.runCollection(ctx, client, spec); err != nil {
			s.emit(Progress{
				Collection: spec.Name,
				Direction:  spec.Direction,
				Phase:      "failed",
				Total:      total,
				Finished:   index,
				Running:    true,
				Error:      err.Error(),
			})
			return err
		}

		s.emit(Progress{
			Collection: spec.Name,
			Direction:  spec.Direction,
			Phase:      "done",
			Total:      total,
			Finished:   index + 1,
			Running:    true,
		})
	}

	return nil
}

// runCollection syncs one collection.
func (s *Service) runCollection(ctx context.Context, client *Client, spec CollectionSpec) error {
	switch spec.Direction {
	case DirectionPush:
		return s.push(ctx, client, spec)
	case DirectionPull:
		return s.pull(ctx, client, spec)
	case DirectionReconcile:
		if err := s.push(ctx, client, spec); err != nil {
			return err
		}
		return s.pull(ctx, client, spec)
	default:
		return fmt.Errorf("sync: unknown direction %q", spec.Direction)
	}
}

// push sends the local records of a collection to the remote server.
func (s *Service) push(ctx context.Context, client *Client, spec CollectionSpec) error {
	cursor, _ := s.Cursors().Get(spec.Name)

	s.emit(Progress{
		Collection: spec.Name,
		Direction:  spec.Direction,
		Phase:      "schema",
		Running:    true,
	})

	// The schema travels first: a record cannot be written into a
	// collection that does not exist.
	if spec.IncludeSchema {
		schema, err := s.store.LocalSchema([]string{spec.Name})
		if err != nil {
			return err
		}
		if err := client.ImportCollections(ctx, schema); err != nil {
			return err
		}
		s.logf("the schema of %s was pushed", spec.Name)
	}

	records, err := s.store.ListRecords(spec.Name, cursor.Push, 0)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		s.logf("%s had nothing to push", spec.Name)
		return nil
	}

	// A record that the other side has newer is not pushed: the
	// push would overwrite the newer record with the older one, and
	// the change would be lost. It is the same rule as the one of a
	// pull, and it is what makes a reconcile a comparison and not
	// two writes in a row.
	remote, err := client.ListRecords(ctx, spec.Name, cursor.Push, 0)
	if err != nil {
		return err
	}

	remoteTime := map[string]time.Time{}
	for _, record := range remote {
		if stamp := record.SourceUpdated(); !stamp.IsZero() {
			remoteTime[record.ID()] = stamp
		}
	}

	toPush := make([]RemoteRecord, 0, len(records))
	skipped := 0
	for _, record := range records {
		if newer, ok := remoteTime[record.ID]; ok && newer.After(record.SourceUpdated) {
			skipped++
			continue
		}
		toPush = append(toPush, record.ToRemote(OriginLocal))
	}

	if skipped > 0 {
		s.logf("%s: %d records were not pushed because the server has a newer one",
			spec.Name, skipped)
	}

	if len(toPush) == 0 {
		s.advanceCursor(spec.Name, DirectionPush, time.Now())
		return nil
	}

	written, err := client.UpsertRecords(ctx, spec.Name, toPush)
	if err != nil {
		return err
	}
	s.logf("%s: %d records were pushed", spec.Name, written)

	// The cursor moves to the time of the run, and not to the time of
	// the newest record: a record that was written while the run was
	// going is still newer than the cursor, and it is picked up by
	// the next run.
	s.advanceCursor(spec.Name, DirectionPush, time.Now())

	return nil
}

// pull takes the remote records of a collection into the local
// database.
func (s *Service) pull(ctx context.Context, client *Client, spec CollectionSpec) error {
	cursor, _ := s.Cursors().Get(spec.Name)

	remote, err := client.ListRecords(ctx, spec.Name, cursor.Pull, 0)
	if err != nil {
		return err
	}
	if len(remote) == 0 {
		s.logf("%s had nothing to pull", spec.Name)
		return nil
	}

	// The records that are newer than the local one win, and the
	// older ones are left alone: that is the whole of the conflict
	// resolution.
	accepted, skipped := s.selectWinners(spec.Name, remote)
	if len(accepted) > 0 {
		written, err := s.store.SaveRecords(spec.Name, accepted, OriginRemote)
		if err != nil {
			return err
		}
		s.logf("%s: %d records were pulled", spec.Name, written)
	}
	if skipped > 0 {
		s.logf("%s: %d records were left alone because the local one is newer", spec.Name, skipped)
	}

	s.advanceCursor(spec.Name, DirectionPull, time.Now())

	return nil
}

// selectWinners picks the remote records that are newer than the
// local ones.
//
// A record that is not here yet is always taken. A record that is
// here is taken when the remote one was changed later, and the
// comparison is made on the field of the sync and not on the
// "updated" field of the server: the server overwrites that one on
// every write, so it says when the server was written to and not when
// the record was changed.
func (s *Service) selectWinners(collection string, remote []RemoteRecord) ([]RemoteRecord, int) {
	accepted := make([]RemoteRecord, 0, len(remote))
	skipped := 0

	for _, record := range remote {
		local, err := s.store.App().FindRecordById(collection, record.ID())
		if err != nil {
			// The record is not here: it is taken.
			accepted = append(accepted, record)
			continue
		}

		localTime := parseDateTime(local.GetString(FieldSourceUpdated))
		remoteTime := record.SourceUpdated()

		// A record that was never stamped is taken: there is
		// nothing to compare it with.
		if localTime.IsZero() || remoteTime.After(localTime) {
			accepted = append(accepted, record)
			continue
		}

		skipped++
	}

	return accepted, skipped
}

// advanceCursor records the time of a run.
func (s *Service) advanceCursor(name string, direction Direction, when time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cursors == nil {
		s.cursors = NewCursorSet()
	}
	cursor := s.cursors.Cursors[name]
	switch direction {
	case DirectionPush:
		cursor.Push = when
	case DirectionPull:
		cursor.Pull = when
	}
	s.cursors.Set(name, cursor)
}

// emit pushes the progress to the interface.
func (s *Service) emit(progress Progress) {
	s.mu.Lock()
	s.progress = progress
	emitter := s.emitter
	s.mu.Unlock()

	if emitter != nil {
		emitter.EmitSyncProgress(progress)
	}
}

// logf records one line of the log of the sync.
func (s *Service) logf(format string, args ...any) {
	s.mu.Lock()
	logger := s.logger
	s.mu.Unlock()

	if logger == nil {
		return
	}
	logger.LogSync(fmt.Sprintf(format, args...))
}

// order sorts the collections so that a relation points at something
// that is already there.
//
// A collection that is pulled is not a dependency of the others: the
// records of the other side are already there, and the order of the
// pull does not matter.
func (s *Service) order(set SyncSet) ([]CollectionSpec, error) {
	dependencies := map[string][]string{}

	for _, spec := range set.Collections {
		collection, err := s.store.App().FindCollectionByNameOrId(spec.Name)
		if err != nil {
			return nil, fmt.Errorf("sync: find the collection %s: %w", spec.Name, err)
		}

		names := make([]string, 0)
		for _, field := range collection.Fields {
			relation, ok := field.(*core.RelationField)
			if !ok || relation.CollectionId == "" {
				continue
			}
			target, err := s.store.App().FindCollectionByNameOrId(relation.CollectionId)
			if err != nil {
				// A relation to a collection that is not there
				// is not a dependency of anything.
				continue
			}
			if target.Name == spec.Name {
				continue
			}
			names = append(names, target.Name)
		}
		dependencies[spec.Name] = names
	}

	ordered := make([]CollectionSpec, 0, len(set.Collections))
	placed := map[string]bool{}

	// The collections are placed one after the other, and a
	// collection is placed before the one that points at it.
	var place func(spec CollectionSpec, seen map[string]bool)
	place = func(spec CollectionSpec, seen map[string]bool) {
		if placed[spec.Name] {
			return
		}
		if seen[spec.Name] {
			// The relations make a circle: there is no order that
			// satisfies a circle, so the collection is left where it
			// is.
			return
		}
		seen[spec.Name] = true

		for _, dependency := range dependencies[spec.Name] {
			next, ok := set.Spec(dependency)
			if !ok {
				// The dependency is not in the sync set: it is
				// already on the other side.
				continue
			}
			place(next, seen)
		}

		if !placed[spec.Name] {
			placed[spec.Name] = true
			ordered = append(ordered, spec)
		}
	}

	for _, spec := range set.Collections {
		place(spec, map[string]bool{})
	}

	return ordered, nil
}
