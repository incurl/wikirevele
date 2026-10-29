package jobs

import (
	"fmt"

	"github.com/pocketbase/dbx"
)

// Reconcile resolves the jobs that a previous run left behind.
//
// A job that was `pending` or `running` when the process died was never
// finished and never failed. Nothing would ever pick it up again, and
// the user could not even delete it: `DeleteJob` refuses to remove a
// job that is still running.
//
// The reconciliation marks them as failed, with an explanation. They
// stay in the history, where the user can see what happened and delete
// them.
//
// It is safe to call more than once: only the jobs that are still
// pending or running are touched.
func (m *Manager) Reconcile() (int, error) {
	records, err := m.app.FindRecordsByFilter(
		CollectionJobs,
		"status = {:status} || status = {:running}",
		"-created",
		0,
		0,
		dbx.Params{"status": StatusPending, "running": StatusRunning},
	)
	if err != nil {
		return 0, fmt.Errorf("find the interrupted jobs: %w", err)
	}

	reconciled := 0
	for _, record := range records {
		record.Set("status", StatusFailed)
		record.Set("error_message", "The job was interrupted by a restart of the application.")

		if err := m.app.Save(record); err != nil {
			return reconciled, fmt.Errorf("mark job %q as failed: %w", record.Id, err)
		}
		reconciled++

		m.emitRecord(record)
		m.logInfo(record.Id, "the job was interrupted by a restart and has been marked as failed")
	}

	return reconciled, nil
}
