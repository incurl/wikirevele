/**
 * The sync panel.
 *
 * It holds the collections that take part in a sync, the credentials
 * of the remote server, and the progress of the run.
 *
 * The run happens in the background: the panel starts it and watches
 * the progress that comes back as a Wails event.
 *
 * The password is write-only from the point of view of the panel: the
 * backend never sends it back, and the panel only ever learns whether
 * one is stored.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';
import { Play, Square } from 'lucide-react';

import * as api from '../../bindings';
import type { SyncCollection, SyncDirection, SyncProgress } from '../../bindings';
import {
  DIRECTION_HINTS,
  DIRECTION_LABELS,
  describeProgress,
  isRunning,
  lastRun,
  needsSyncFields,
  problemsOf,
  selectedCount,
} from './sync';

/**
 * The props of the sync panel.
 */
export interface SyncPanelProps {
  /** Called with a message to show to the user. */
  readonly onNotify?: (message: string, kind?: 'info' | 'error' | 'success') => void;
}

/**
 * The sync panel.
 */
export function SyncPanel({ onNotify }: SyncPanelProps) {
  const queryClient = useQueryClient();

  const [baseUrl, setBaseUrl] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [collections, setCollections] = useState<SyncCollection[]>([]);
  const [progress, setProgress] = useState<SyncProgress | null>(null);
  const [loaded, setLoaded] = useState(false);

  // The set of the collections and the credentials come from the
  // backend, which is the only one that holds them.
  const setQuery = useQuery({
    queryKey: ['sync-set'],
    queryFn: () => api.getSyncSet(),
  });

  useEffect(() => {
    if (setQuery.data === undefined || loaded) {
      return;
    }
    setBaseUrl(setQuery.data.base_url ?? '');
    setEmail(setQuery.data.email ?? '');
    setCollections(setQuery.data.collections);
    setLoaded(true);
  }, [setQuery.data, loaded]);

  // The progress of the run is pushed as a Wails event.
  useEffect(() => {
    const unsubscribe = Events.On(api.EVENT_SYNC_PROGRESS, (event) => {
      // The payload of an event is in its data, and it may be
      // missing: the event is a notification, not a guarantee.
      const payload = event.data as SyncProgress | undefined;
      if (!payload) {
        return;
      }
      setProgress(payload);

      if (payload.over) {
        // The run is over: the times of the last runs changed.
        void queryClient.invalidateQueries({ queryKey: ['sync-set'] });
      }
    });

    return () => {
      unsubscribe();
    };
  }, [queryClient]);

  // The state of the last run is read when the panel opens, so that
  // it shows the outcome of the run that ran before it was opened.
  useEffect(() => {
    let cancelled = false;
    void api
      .getSyncProgress()
      .then((state) => {
        if (!cancelled) {
          setProgress(state);
        }
      })
      .catch(() => {
        // The panel opens without a progress: the run has not
        // happened yet.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const hasPassword = setQuery.data?.has_password ?? false;

  const problems = useMemo(
    () => problemsOf(collections, baseUrl, email, hasPassword),
    [collections, baseUrl, email, hasPassword],
  );

  const update = useCallback((name: string, change: Partial<SyncCollection>) => {
    setCollections((current) =>
      current.map((collection) =>
        collection.name === name ? { ...collection, ...change } : collection,
      ),
    );
  }, []);

  const saveMutation = useMutation({
    mutationFn: async () => {
      await api.saveSyncSet({
        base_url: baseUrl.trim(),
        email: email.trim(),
        has_password: hasPassword,
        configured: setQuery.data?.configured ?? false,
        collections,
      });
      if (password.trim()) {
        await api.setSyncPassword(password.trim());
      }
    },
    onSuccess: () => {
      setPassword('');
      onNotify?.('The sync set was saved.', 'success');
      void queryClient.invalidateQueries({ queryKey: ['sync-set'] });
    },
    onError: (error: Error) => {
      onNotify?.(error.message, 'error');
    },
  });

  const startMutation = useMutation({
    mutationFn: () => api.startSync(),
    onSuccess: () => {
      onNotify?.('The sync started.', 'info');
    },
    onError: (error: Error) => {
      onNotify?.(error.message, 'error');
    },
  });

  const cancelMutation = useMutation({
    mutationFn: () => api.cancelSync(),
    onError: (error: Error) => {
      onNotify?.(error.message, 'error');
    },
  });

  const running = isRunning(progress);

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Sync</span>
        {running && <span className="text-dim">running</span>}
      </div>

      <div className="panel-body">
        <div className="field">
          <label className="field-label" htmlFor="sync-url">
            Server address
          </label>
          <input
            id="sync-url"
            className="input"
            type="text"
            value={baseUrl}
            placeholder="https://ruis.example.com"
            onChange={(event) => setBaseUrl(event.target.value)}
          />
          <span className="field-hint">
            The root of the remote server. The API is under it, at /api.
          </span>
        </div>

        <div className="field">
          <label className="field-label" htmlFor="sync-email">
            Superuser
          </label>
          <input
            id="sync-email"
            className="input"
            type="text"
            value={email}
            placeholder="admin@example.com"
            onChange={(event) => setEmail(event.target.value)}
          />
        </div>

        <div className="field">
          <label className="field-label" htmlFor="sync-password">
            Password
          </label>
          <input
            id="sync-password"
            className="input"
            type="password"
            placeholder={
              hasPassword ? 'A password is stored. Leave empty to keep it.' : 'The password'
            }
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
          <span className="field-hint">
            {hasPassword
              ? 'A password is stored. It is kept encrypted and is never sent back to this window.'
              : 'No password is stored. The backend reads it, never the frontend.'}
          </span>
        </div>

        <div className="field">
          <span className="field-label">
            Collections ({selectedCount(collections)} selected)
          </span>
          <span className="field-hint">
            The collections of the local database. The system collections are not offered: they
            belong to PocketBase.
          </span>

          {collections.length === 0 ? (
            <span className="field-hint">No collection can be synced.</span>
          ) : (
            <div className="sync-list">
              {collections.map((collection) => (
                <div key={collection.name} className="sync-row">
                  <label className="checkbox">
                    <input
                      type="checkbox"
                      checked={collection.selected}
                      onChange={(event) =>
                        update(collection.name, { selected: event.target.checked })
                      }
                    />
                    <span>{collection.name}</span>
                  </label>

                  <span className="text-dim">{collection.records} records</span>

                  <select
                    aria-label={`Direction of ${collection.name}`}
                    title={DIRECTION_HINTS[collection.direction]}
                    value={collection.direction}
                    disabled={!collection.selected}
                    onChange={(event) =>
                      update(collection.name, {
                        direction: event.target.value as SyncDirection,
                      })
                    }
                  >
                    {(Object.keys(DIRECTION_LABELS) as SyncDirection[]).map((direction) => (
                      <option key={direction} value={direction}>
                        {DIRECTION_LABELS[direction]}
                      </option>
                    ))}
                  </select>

                  <label className="checkbox">
                    <input
                      type="checkbox"
                      checked={collection.include_schema}
                      disabled={!collection.selected || collection.direction === 'pull'}
                      onChange={(event) =>
                        update(collection.name, { include_schema: event.target.checked })
                      }
                    />
                    <span>schema</span>
                  </label>

                  <span className="text-dim">
                    {lastRun(collection.last_push)} / {lastRun(collection.last_pull)}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>

        {needsSchemaHint(collections) && (
          <span className="field-hint">
            A collection without the fields of the sync gets them when the run starts: the fields
            hold the time of the change of a record, and they are what the two sides are compared
            on.
          </span>
        )}

        {problems.length > 0 && (
          <ul className="sync-problems">
            {problems.map((problem) => (
              <li key={problem}>{problem}</li>
            ))}
          </ul>
        )}

        <div className="field-row">
          <button
            type="button"
            className="btn"
            onClick={() => saveMutation.mutate()}
            disabled={saveMutation.isPending}
          >
            Save
          </button>
          {running ? (
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => cancelMutation.mutate()}
            >
              <Square size={13} /> Stop
            </button>
          ) : (
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => startMutation.mutate()}
              disabled={problems.length > 0 || startMutation.isPending}
            >
              <Play size={13} /> Sync now
            </button>
          )}
        </div>

        <div className="field">
          <span className="field-label">Progress</span>
          <span className="field-hint">{describeProgress(progress)}</span>
        </div>
      </div>
    </section>
  );
}

/**
 * Tells whether one of the collections is missing the fields of the
 * sync.
 */
function needsSchemaHint(collections: SyncCollection[]): boolean {
  return collections.some((collection) => collection.selected && needsSyncFields(collection));
}
