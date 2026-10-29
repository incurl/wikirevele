package jobs

import (
	"context"
	"time"
)

// Shutdown stops the manager.
//
// The running jobs are cancelled and, if the context allows it, waited
// for. A job that is still running when the wait expires is abandoned:
// the process is going away anyway.
//
// The jobs that were running are marked as failed, so that the next
// start finds no half-finished job.
//
// Shutdown is safe to call more than once.
func (m *Manager) Shutdown(ctx context.Context) {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.cancels))
	for _, cancel := range m.cancels {
		cancels = append(cancels, cancel)
	}
	m.mu.Unlock()

	// Cancel the root context, which stops every job that has not
	// finished yet.
	m.rootCancel()

	for _, cancel := range cancels {
		cancel()
	}

	// Wait for the running jobs to finish.
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		// The wait expired: the jobs are abandoned.
		m.logInfo("manager", "the shutdown gave up waiting for the running jobs")
	}
}

// ShutdownTimeout is the time a shutdown waits for the running jobs
// before abandoning them.
const ShutdownTimeout = 5 * time.Second

// ShutdownWithTimeout stops the manager, waiting at most timeout for
// the running jobs.
//
// It is the convenient form for an application that has no context of
// its own to hand.
func (m *Manager) ShutdownWithTimeout(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	m.Shutdown(ctx)
}
