/**
 * The jobs rail of the application.
 *
 * It is the right rail, and it holds one pane: the queue of the jobs.
 * It collapses as a whole, so that the view can take the room when the
 * queue is not what the user is looking at.
 *
 * A collapsed rail is a strip with its icon and the number of the jobs
 * that are moving, so that the queue is still readable at a glance
 * without taking a panel's width.
 */

import { PanelRightClose, PanelRightOpen } from "lucide-react";

import { JobQueue } from "../features/queue/JobQueue";
import type { Job } from "../bindings";

/**
 * The props of the jobs rail.
 */
export interface JobsRailProps {
  /** The recorded jobs, newest first. */
  readonly jobs: readonly Job[];
  /** Whether the rail is collapsed. */
  readonly collapsed: boolean;
  /** Called when the user collapses or expands the rail. */
  readonly onToggle: () => void;
  /** Cancels a pending or running job. */
  readonly onCancel: (jobId: string) => void;
  /** Runs a job again with the same parameters. */
  readonly onRetry: (jobId: string) => void;
  /** Removes a finished job from the queue. */
  readonly onDelete: (jobId: string) => void;
}

/**
 * The jobs rail.
 */
export function JobsRail({ jobs, collapsed, onToggle, onCancel, onRetry, onDelete }: JobsRailProps) {
  const moving = jobs.filter((job) => job.status === "running" || job.status === "pending").length;

  if (collapsed) {
    return (
      <aside className="app-jobs app-jobs-collapsed" aria-label="Jobs">
        <button
          type="button"
          className="jobs-toggle"
          onClick={onToggle}
          aria-expanded={false}
          title="Show the jobs"
        >
          <PanelRightOpen className="navigator-icon" aria-hidden="true" />
          {moving > 0 ? <span className="jobs-count">{moving}</span> : null}
          <span className="jobs-collapsed-label">Jobs</span>
        </button>
      </aside>
    );
  }

  return (
    <aside className="app-jobs" aria-label="Jobs">
      <div className="jobs-header">
        <span className="jobs-title">
          Jobs
          {moving > 0 ? <span className="jobs-count">{moving}</span> : null}
        </span>

        <button
          type="button"
          className="jobs-toggle"
          onClick={onToggle}
          aria-expanded
          title="Hide the jobs"
        >
          <PanelRightClose className="navigator-icon" aria-hidden="true" />
        </button>
      </div>

      <JobQueue jobs={jobs as Job[]} onCancel={onCancel} onRetry={onRetry} onDelete={onDelete} />
    </aside>
  );
}
