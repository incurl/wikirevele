/**
 * The Wikirevele application shell.
 *
 * The layout is a left control rail, a centre canvas and a right
 * sidebar, with the job queue in the header area.
 *
 * The debug console sits at the bottom of the window. It is hidden
 * until it is asked for, with Ctrl+Shift+I or with the button in the
 * header, so that it does not take the room of the panels above it.
 *
 * All backend access goes through the Wails bindings; the frontend never
 * talks to PocketBase or to a remote API directly.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';

import '../styles.css';
// The shadcn/ui theme: the Tailwind layers and the CSS variables the
// components use. It is imported here so that the whole interface has
// the same theme.
import '../styles/tailwind.css';

import * as api from '../bindings';
import type { Job } from '../bindings';
import { applyZoom } from '../ui/zoom';
import { isConsoleShortcut } from '../ui/shortcuts';
import { ConsolePanel, countErrors } from '../features/console/ConsolePanel';
import { EtlHistoryPanel } from '../features/etl/EtlHistoryPanel';
import { EtlPanel } from '../features/etl/EtlPanel';
import { JobQueue } from '../features/queue/JobQueue';
import { SettingsDialog } from '../features/settings/SettingsDialog';
import { SubmitPanel } from '../features/submit/SubmitPanel';
import { WikidataPanel } from '../features/wikidata/WikidataPanel';

interface Toast {
  message: string;
  kind: 'info' | 'error' | 'success';
}

/**
 * The application root.
 */
function App() {
  const queryClient = useQueryClient();

  const [settingsOpen, setSettingsOpen] = useState(false);
  const [consoleOpen, setConsoleOpen] = useState(false);
  const [toast, setToast] = useState<Toast | null>(null);

  // The toast timer is kept so that a second toast replaces the first
  // instead of being dismissed by the first one's timer.
  const toastTimer = useRef<number | null>(null);

  const notify = useCallback((message: string, kind: Toast['kind'] = 'info') => {
    setToast({ message, kind });

    if (toastTimer.current !== null) {
      window.clearTimeout(toastTimer.current);
    }
    toastTimer.current = window.setTimeout(() => {
      toastTimer.current = null;
      setToast(null);
    }, 4000);
  }, []);

  // A pending timer must not outlive the component.
  useEffect(() => {
    return () => {
      if (toastTimer.current !== null) {
        window.clearTimeout(toastTimer.current);
      }
    };
  }, []);

  // --- Data ---------------------------------------------------------

  const settingsQuery = useQuery({
    queryKey: ['settings'],
    queryFn: () => api.getSettings(),
  });

  const jobsQuery = useQuery({
    queryKey: ['jobs'],
    queryFn: () => api.listJobs(50),
    // The Wails event pushes updates; this interval is the fallback.
    refetchInterval: 5000,
  });

  const consoleQuery = useQuery({
    queryKey: ['console'],
    queryFn: () => api.listConsoleEntries(),
    refetchInterval: 5000,
  });

  // --- Live updates -------------------------------------------------

  // The job event keeps the queue in step with the backend without
  // polling. The refetch interval above is only the safety net, so a
  // missed event only delays a refresh.
  useEffect(() => {
    const unsubscribe = Events.On(api.EVENT_JOB_UPDATED, () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
    });
    return () => {
      unsubscribe();
    };
  }, [queryClient]);

  useEffect(() => {
    const unsubscribe = Events.On(api.EVENT_CONSOLE_ENTRY, () => {
      void queryClient.invalidateQueries({ queryKey: ['console'] });
    });
    return () => {
      unsubscribe();
    };
  }, [queryClient]);

  // The debug console is shown and hidden with Ctrl+Shift+I. The
  // shortcut is handled in the page, because the webview has no menu
  // bar of its own.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (!isConsoleShortcut(event)) {
        return;
      }
      // The webview would otherwise open its own inspector.
      event.preventDefault();
      setConsoleOpen((value) => !value);
    };

    window.addEventListener('keydown', onKeyDown);
    return () => {
      window.removeEventListener('keydown', onKeyDown);
    };
  }, []);

  // --- Actions ------------------------------------------------------

  const submitMutation = useMutation({
    mutationFn: (request: { prompt: string; params?: Record<string, unknown> }) =>
      api.submitJob(request),
    onSuccess: (job: Job) => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      notify(`The job "${job.title}" was queued.`, 'success');
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const cancelMutation = useMutation({
    mutationFn: (jobId: string) => api.cancelJob(jobId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const retryMutation = useMutation({
    mutationFn: (jobId: string) => api.retryJob(jobId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      notify('The job was queued again.', 'success');
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (jobId: string) => api.deleteJob(jobId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
    onError: (error: Error) => {
      notify(error.message, 'error');
    },
  });

  const clearConsoleMutation = useMutation({
    mutationFn: () => api.clearConsoleEntries(),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['console'] });
    },
  });

  // The credentials and the interface preferences are saved through
  // mutations, so that a failure is reported instead of becoming an
  // unhandled rejection.
  const saveCredentialsMutation = useMutation({
    mutationFn: ({ apiKey, baseUrl }: { apiKey: string; baseUrl: string }) =>
      api.saveCredentials(apiKey, baseUrl),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['settings'] });
      notify('The settings were saved.', 'success');
      setSettingsOpen(false);
    },
    onError: (error: Error) => {
      notify(`The settings could not be saved: ${error.message}`, 'error');
    },
  });

  const saveUIMutation = useMutation({
    mutationFn: (ui: { zoom: number; theme: string }) => api.saveUI(ui),
    onSuccess: (_data, ui) => {
      void queryClient.invalidateQueries({ queryKey: ['settings'] });
      applyZoom(ui.zoom);
    },
    onError: (error: Error) => {
      notify(`The zoom could not be saved: ${error.message}`, 'error');
    },
  });

  // The zoom is restored as soon as the settings are known.
  useEffect(() => {
    if (settingsQuery.data) {
      applyZoom(settingsQuery.data.ui.zoom);
    }
  }, [settingsQuery.data]);

  const jobs = jobsQuery.data ?? [];
  const entries = consoleQuery.data ?? [];
  const consoleErrors = countErrors(entries);

  return (
    <div className="app">
      <header className="app-header">
        <span className="app-brand">
          Wikirevele
          <small>Wails v3 · PocketBase · React</small>
        </span>

        <span className="app-header-spacer" />

        <JobQueue
          jobs={jobs}
          onCancel={(jobId) => cancelMutation.mutate(jobId)}
          onRetry={(jobId) => retryMutation.mutate(jobId)}
          onDelete={(jobId) => deleteMutation.mutate(jobId)}
        />

        <button
          type="button"
          className={`btn btn-sm${consoleOpen ? ' btn-active' : ''}`}
          onClick={() => setConsoleOpen((value) => !value)}
          title="Show or hide the debug console (Ctrl+Shift+I)"
          aria-pressed={consoleOpen}
        >
          Console
          {consoleErrors > 0 && (
            <span className="console-error-count"> · {consoleErrors}</span>
          )}
        </button>

        <button type="button" className="btn btn-sm" onClick={() => setSettingsOpen(true)}>
          Settings
        </button>
      </header>

      <aside className="app-rail">
        <EtlPanel onNotify={(message, kind) => notify(message, kind)} />
        <SubmitPanel
          disabled={submitMutation.isPending}
          onSubmit={(prompt, params) => submitMutation.mutate({ prompt, params })}
        />
      </aside>

      <main className="app-canvas">
        <WikidataPanel />

        <section className="panel">
          <div className="panel-header">
            <span>Getting started</span>
          </div>
          <div className="panel-body">
            <p className="text-dim">
              This is the Wikirevele scaffold. It ships a generic job queue, an
              embedded PocketBase and a React shell. Replace the runner in
              <code> internal/runner</code> with your own backend, then extend
              the panels to match.
            </p>
            <p className="text-dim">
              Submit a prompt on the left to see the whole pipeline work: the
              job is persisted, queued, executed by the runner and pushed back
              to this window as a Wails event.
            </p>
          </div>
        </section>
      </main>

      <aside className="app-gallery">
        <EtlHistoryPanel onNotify={(message, kind) => notify(message, kind)} />

        <section className="panel">
          <div className="panel-header">
            <span>Connection</span>
          </div>
          <div className="panel-body">
            <div className="field">
              <span className="field-label">API key</span>
              <span className="text-dim">
                {settingsQuery.data?.has_api_key
                  ? 'Configured. It is stored encrypted and never sent back to the UI.'
                  : 'Not configured. Add one in Settings.'}
              </span>
            </div>
            <div className="field">
              <span className="field-label">Base URL</span>
              <span className="text-dim">{settingsQuery.data?.base_url ?? '—'}</span>
            </div>
          </div>
        </section>
      </aside>

      {consoleOpen && (
        <ConsolePanel
          entries={entries}
          onClear={() => clearConsoleMutation.mutate()}
        />
      )}

      <SettingsDialog
        open={settingsOpen}
        settings={settingsQuery.data ?? null}
        onClose={() => setSettingsOpen(false)}
        onSave={(apiKey, baseUrl) => {
          saveCredentialsMutation.mutate({ apiKey, baseUrl });
        }}
        onSaveUI={(ui) => {
          saveUIMutation.mutate(ui);
        }}
      />

      {toast && (
        <div className={`toast toast-${toast.kind}`} role="status">
          {toast.message}
        </div>
      )}
    </div>
  );
}

export default App;
