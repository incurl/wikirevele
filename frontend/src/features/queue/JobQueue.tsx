/**
 * The job queue.
 *
 * A live list of jobs with status indicators, elapsed time and per-job
 * actions (cancel, retry, delete).
 */

import { useMemo } from 'react';

import type { Job } from '../../bindings';

interface JobQueueProps {
  /** The recorded jobs, newest first. */
  jobs: Job[];
  /** Cancels a pending or running job. */
  onCancel: (jobId: string) => void;
  /** Runs a job again with the same parameters. */
  onRetry: (jobId: string) => void;
  /** Removes a finished job from the queue. */
  onDelete: (jobId: string) => void;
}

/**
 * Reports whether a job has come to rest and can be removed.
 *
 * A pending or running job has to be cancelled first, so it cannot
 * write to the database after it has been removed.
 */
export function canDelete(status: Job['status']): boolean {
  return status === 'completed' || status === 'failed' || status === 'cancelled';
}

/**
 * Formats the elapsed time since a job was created.
 */
export function elapsed(created: string): string {
  if (!created) {
    return '';
  }
  const start = new Date(created).getTime();
  if (Number.isNaN(start)) {
    return '';
  }
  const seconds = Math.max(0, Math.round((Date.now() - start) / 1000));
  if (seconds < 60) {
    return `${seconds}s`;
  }
  const minutes = Math.floor(seconds / 60);
  return `${minutes}m ${seconds % 60}s`;
}

/**
 * The job queue panel.
 */
export function JobQueue({ jobs, onCancel, onRetry, onDelete }: JobQueueProps) {
  const summary = useMemo(() => {
    const running = jobs.filter((job) => job.status === 'running').length;
    const pending = jobs.filter((job) => job.status === 'pending').length;
    return { running, pending };
  }, [jobs]);

  const recent = jobs.slice(0, 8);

  return (
    <section className="queue">
      <span className="queue-summary">
        {summary.running} running · {summary.pending} pending
      </span>

      {recent.length > 0 && (
        <ul className="queue-list">
          {recent.map((job) => (
            <li key={job.id} className={`queue-item queue-status-${job.status}`}>
              <span className="queue-dot" title={job.status} />
              <span className="queue-title" title={job.title}>
                {job.title}
              </span>
              <span className="queue-elapsed">{elapsed(job.created)}</span>

              {(job.status === 'pending' || job.status === 'running') && (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => onCancel(job.id)}
                  title="Cancel this job"
                >
                  Cancel
                </button>
              )}

              {job.status === 'failed' && (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => onRetry(job.id)}
                  title="Run this job again"
                >
                  Retry
                </button>
              )}

              {canDelete(job.status) && (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => onDelete(job.id)}
                  title="Remove this job from the queue"
                >
                  ×
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
