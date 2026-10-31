/**
 * The WikiRevele application shell.
 *
 * The layout is a navigator on the left, the view it switches in the
 * centre, and the job queue in a rail on the right.
 *
 * The debug console sits at the bottom of the window. It is hidden
 * until it is asked for, with Ctrl+Shift+I or with the button in the
 * header, so that it does not take the room of the panels above it.
 *
 * All backend access goes through the Wails bindings; the frontend never
 * talks to PocketBase or to a remote API directly.
 */

import { useCallback, useEffect, useRef, useState } from 'react';
import { useLocation } from 'react-router';
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
import { JobsRail } from './JobsRail';
import { Navigator } from './Navigator';
import { DEFAULT_SCOPE, SCOPES, scopeOfPath, type ScopeId } from './scopes';
import { SettingsDialog } from '../features/settings/SettingsDialog';
import { SubmitPanel } from '../features/submit/SubmitPanel';
import { WikidataPanel } from '../features/wikidata/WikidataPanel';
import { ImportPanel } from '../features/wikidata/components/ImportPanel';

/**
 * The accessible name of each view.
 */
const SCOPE_LABELS: Record<ScopeId, string> = Object.fromEntries(
  SCOPES.map((scope) => [scope.id, scope.label]),
) as Record<ScopeId, string>;

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

  // The scope the view shows, and whether the two rails are giving
  // their room back to it.
  const [scope, setScope] = useState<ScopeId>(DEFAULT_SCOPE);
  const [navigatorCollapsed, setNavigatorCollapsed] = useState(false);
  const [jobsCollapsed, setJobsCollapsed] = useState(false);

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

  // A route of the workarea switches the view to it. A link to an item
  // changes the path, and the page it addresses has to be shown: a
  // navigator that stayed on another scope would address a page that is
  // not on screen.
  const location = useLocation();

  useEffect(() => {
    const wanted = scopeOfPath(location.pathname);
    if (wanted) {
      setScope(wanted);
    }
  }, [location.pathname]);

  const jobs = jobsQuery.data ?? [];
  const entries = consoleQuery.data ?? [];
  const consoleErrors = countErrors(entries);

  return (
    <div className="app">
      <header className="app-header">
        <span className="app-brand">
          WikiRevele
          <small>Wails v3 · PocketBase · React</small>
        </span>

        <span className="app-header-spacer" />

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

      {/*
        The navigator switches the view. It holds no pane of its own:
        every scope is rendered in the view, so each of them has the
        whole width of the window.
      */}
      <Navigator
        active={scope}
        onSelect={setScope}
        collapsed={navigatorCollapsed}
        onToggle={() => setNavigatorCollapsed((value) => !value)}
      />

      <main className="app-view" aria-label={SCOPE_LABELS[scope]}>
        {scope === 'workarea' && <WikidataPanel />}
        {scope === 'extract' && <EtlPanel onNotify={(message, kind) => notify(message, kind)} />}
        {scope === 'import' && <ImportPanel />}
        {scope === 'submit' && (
          <SubmitPanel
            disabled={submitMutation.isPending}
            onSubmit={(prompt, params) => submitMutation.mutate({ prompt, params })}
          />
        )}
        {scope === 'history' && <EtlHistoryPanel onNotify={(message, kind) => notify(message, kind)} />}
      </main>

      <JobsRail
        jobs={jobs}
        collapsed={jobsCollapsed}
        onToggle={() => setJobsCollapsed((value) => !value)}
        onCancel={(jobId) => cancelMutation.mutate(jobId)}
        onRetry={(jobId) => retryMutation.mutate(jobId)}
        onDelete={(jobId) => deleteMutation.mutate(jobId)}
      />

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
