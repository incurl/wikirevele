/**
 * The pure logic of the sync panel.
 *
 * The panel is a form: it holds the set of the collections, the
 * credentials and the progress of the run. Everything that can be
 * decided without asking the backend is decided here, and it is
 * tested here.
 */

import type { SyncCollection, SyncDirection, SyncProgress } from '../../bindings';

/**
 * The directions a collection can be synced in, with the labels the
 * panel shows.
 */
export const DIRECTION_LABELS: Record<SyncDirection, string> = {
  push: 'Push',
  pull: 'Pull',
  reconcile: 'Reconcile',
};

/**
 * The text that explains what a direction does.
 *
 * The reader has to know what a direction does before choosing it, so
 * the explanation is next to the choice and not in a help page.
 */
export const DIRECTION_HINTS: Record<SyncDirection, string> = {
  push: 'The local records are sent to the server.',
  pull: 'The records of the server are taken into the local database.',
  reconcile: 'Both sides are compared, and the newer record wins.',
};

/**
 * The name of a direction, as the backend stores it.
 *
 * The direction travels as a string, and the panel has to show the
 * name of the one it was given.
 */
export function directionLabel(direction: string): string {
  if (direction in DIRECTION_LABELS) {
    return DIRECTION_LABELS[direction as SyncDirection];
  }
  return direction;
}

/**
 * The number of the collections of a set that take part in a sync.
 */
export function selectedCount(collections: SyncCollection[]): number {
  return collections.filter((collection) => collection.selected).length;
}

/**
 * The problems of the set, shown before a run starts.
 *
 * A set that cannot be run is refused by the backend as well, and the
 * panel says why before the reader presses the button.
 */
export function problemsOf(
  collections: SyncCollection[],
  baseUrl: string,
  email: string,
  hasPassword: boolean,
): string[] {
  const found: string[] = [];

  if (selectedCount(collections) === 0) {
    found.push('At least one collection must be selected.');
  }
  if (!baseUrl.trim()) {
    found.push('The address of the server is required.');
  }
  if (!email.trim()) {
    found.push('The identity of the superuser is required.');
  }
  if (!hasPassword) {
    found.push('The password of the superuser is required.');
  }

  return found;
}

/**
 * Tells whether a sync is running.
 *
 * The progress is the only sign of it, and the progress of a run
 * that has not started yet says that it is over.
 */
export function isRunning(progress: SyncProgress | null): boolean {
  return progress !== null && progress.running;
}

/**
 * The text of the progress, as the panel shows it.
 *
 * The reader wants to know what the run is doing and not the shape of
 * the progress.
 */
export function describeProgress(progress: SyncProgress | null): string {
  if (progress === null) {
    return 'No sync has run yet.';
  }

  if (progress.error) {
    return `The sync failed: ${progress.error}`;
  }

  if (progress.running) {
    if (progress.collection) {
      return `${progress.collection}: ${progress.phase} (${progress.finished}/${progress.total})`;
    }
    return `Running (${progress.finished}/${progress.total})`;
  }

  if (progress.over) {
    return `The sync is over. ${progress.total} collection(s) were synced.`;
  }

  return 'The sync is idle.';
}

/**
 * The time of the last run of a collection, as the panel shows it.
 *
 * A collection that was never synced says "never".
 */
export function lastRun(value: string): string {
  return value.trim() ? value : 'never';
}

/**
 * Tells whether a collection can be synced.
 *
 * A collection without the fields of the sync gets them when the run
 * starts, so it can be synced: the panel says so, because it is
 * worth knowing that the schema of the collection will change.
 */
export function needsSyncFields(collection: SyncCollection): boolean {
  return !collection.has_sync_field;
}
