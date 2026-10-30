/**
 * The ETL panel.
 *
 * It collects the parameters of a run, shows the shell pipeline that
 * the parameters produce, and runs it.
 *
 * The command is the important part of the panel. It is what the
 * application runs, and it is what the user copies to run the same
 * job in a terminal: there is only one of them, so the copy always
 * does what the button did.
 *
 * The run itself happens in a child process: the panel starts it and
 * watches the progress that comes back as a Wails event.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Events } from '@wailsio/runtime';
import { Copy, Play, Square } from 'lucide-react';

import * as api from '../../bindings';
import type { EtlProgress } from '../../bindings';
import { writeClipboard } from '../console/ConsolePanel';
import {
  formatCount,
  formatSpeed,
  isLanguageCode,
  isQID,
  isRunning,
  parseLanguages,
  statusLabel,
} from './command';

/**
 * The props of the ETL panel.
 */
export interface EtlPanelProps {
  /** Called with a message to show to the user. */
  readonly onNotify?: (message: string, kind?: 'info' | 'error' | 'success') => void;
}

/**
 * The defaults of the panel.
 *
 * The output files are in the data directory of the application, which
 * is where the import panel looks for them.
 */
const DEFAULT_TARGET = 'Q90';
const DEFAULT_LANGS = 'en,fr';
const DEFAULT_ITEMS = 'paris-wikidata-subset.ndjson';
const DEFAULT_PROPS = 'wikidata-properties.ndjson';

/**
 * The ETL panel.
 */
export function EtlPanel({ onNotify }: EtlPanelProps) {
  const queryClient = useQueryClient();

  const [dumpPath, setDumpPath] = useState('');
  const [target, setTarget] = useState(DEFAULT_TARGET);
  const [recursive, setRecursive] = useState(false);
  const [langs, setLangs] = useState(DEFAULT_LANGS);
  const [itemsPath, setItemsPath] = useState(DEFAULT_ITEMS);
  const [propsPath, setPropsPath] = useState(DEFAULT_PROPS);
  const [overwrite, setOverwrite] = useState(false);

  const [command, setCommand] = useState('');
  const [commandError, setCommandError] = useState<string | null>(null);
  const [progress, setProgress] = useState<EtlProgress | null>(null);
  const [copied, setCopied] = useState(false);

  // The options of the run, as the backend takes them.
  const options = useMemo(
    () => ({
      dump_path: dumpPath,
      target_qid: target.trim(),
      recursive_p131: recursive,
      langs: parseLanguages(langs),
      out_items_path: itemsPath.trim(),
      out_props_path: propsPath.trim(),
      overwrite,
    }),
    [dumpPath, target, recursive, langs, itemsPath, propsPath, overwrite],
  );

  // The problems of the parameters, shown before the run starts.
  const problems = useMemo(() => {
    const found: string[] = [];
    if (!isQID(target)) {
      found.push('The target must be a Wikidata identifier, such as Q90.');
    }
    for (const language of parseLanguages(langs)) {
      if (!isLanguageCode(language)) {
        found.push(`The language "${language}" is not a language code.`);
      }
    }
    if (!itemsPath.trim() || !propsPath.trim()) {
      found.push('The two output files are required.');
    }
    if (itemsPath.trim() && itemsPath.trim() === propsPath.trim()) {
      found.push('The items and the properties cannot be written to the same file.');
    }
    return found;
  }, [target, langs, itemsPath, propsPath]);

  // The command is built by the backend, so that it is exactly the
  // command the application will run.
  const commandQuery = useQuery({
    queryKey: ['etl-command', options],
    queryFn: () => api.buildEtlCommand(options),
    enabled: problems.length === 0 && dumpPath.trim().length > 0,
  });

  useEffect(() => {
    if (commandQuery.data !== undefined) {
      setCommand(commandQuery.data);
      setCommandError(null);
    }
  }, [commandQuery.data]);

  useEffect(() => {
    if (commandQuery.error) {
      setCommand('');
      setCommandError(String((commandQuery.error as Error).message ?? commandQuery.error));
    }
  }, [commandQuery.error]);

  // The progress of the run is pushed as a Wails event.
  useEffect(() => {
    const unsubscribe = Events.On(api.EVENT_ETL_PROGRESS, (event) => {
      // The payload of an event is in its data, and it may be
      // missing: the event is a notification, not a guarantee.
      const payload = event.data as EtlProgress | undefined;
      if (!payload) {
        return;
      }
      setProgress(payload);

      if (!isRunning(payload.status)) {
        // The run is over: the history and the imported dataset have
        // to be read again.
        void queryClient.invalidateQueries({ queryKey: ['etl-history'] });
        void queryClient.invalidateQueries({ queryKey: ['wikidata'] });
      }
    });

    return () => {
      unsubscribe();
    };
  }, [queryClient]);

  const startMutation = useMutation({
    mutationFn: () => api.startEtlJob(options),
    onSuccess: (state) => {
      setProgress(state);
      onNotify?.('The job started.', 'info');
    },
    onError: (error: Error) => {
      onNotify?.(error.message, 'error');
    },
  });

  const cancelMutation = useMutation({
    mutationFn: (runId: string) => api.cancelEtlJob(runId),
    onError: (error: Error) => {
      onNotify?.(error.message, 'error');
    },
  });

  const copy = useCallback(async () => {
    const ok = await writeClipboard(command);
    if (!ok) {
      onNotify?.('The command could not be copied.', 'error');
      return;
    }
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1500);
  }, [command, onNotify]);

  const running = progress !== null && isRunning(progress.status);

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Extract</span>
        {running && <span className="text-dim">running</span>}
      </div>

      <div className="panel-body">
        <div className="field">
          <label className="field-label" htmlFor="etl-dump">
            Dump file
          </label>
          <input
            id="etl-dump"
            className="input"
            type="text"
            value={dumpPath}
            placeholder="/path/to/latest-all.json.bz2"
            onChange={(event) => setDumpPath(event.target.value)}
          />
          <span className="field-hint">
            The compressed dump. The decompressor must be on the host: the application
            does not ship it.
          </span>
        </div>

        <div className="field">
          <label className="field-label" htmlFor="etl-target">
            Target
          </label>
          <input
            id="etl-target"
            className="input"
            type="text"
            value={target}
            placeholder="Q90"
            onChange={(event) => setTarget(event.target.value)}
          />
          <span className="field-hint">The Wikidata identifier the items are linked to.</span>
        </div>

        <div className="field">
          <label className="field-label" htmlFor="etl-langs">
            Languages
          </label>
          <input
            id="etl-langs"
            className="input"
            type="text"
            value={langs}
            placeholder="en,fr"
            onChange={(event) => setLangs(event.target.value)}
          />
          <span className="field-hint">
            The languages of the labels, descriptions and aliases that are kept.
          </span>
        </div>

        <div className="field">
          <label className="field-label" htmlFor="etl-items">
            Items output
          </label>
          <input
            id="etl-items"
            className="input"
            type="text"
            value={itemsPath}
            onChange={(event) => setItemsPath(event.target.value)}
          />
        </div>

        <div className="field">
          <label className="field-label" htmlFor="etl-props">
            Properties output
          </label>
          <input
            id="etl-props"
            className="input"
            type="text"
            value={propsPath}
            onChange={(event) => setPropsPath(event.target.value)}
          />
        </div>

        <div className="field field-row">
          <label className="checkbox">
            <input
              type="checkbox"
              checked={recursive}
              onChange={(event) => setRecursive(event.target.checked)}
            />
            <span>Include everything inside the target (P131)</span>
          </label>
        </div>

        <div className="field field-row">
          <label className="checkbox">
            <input
              type="checkbox"
              checked={overwrite}
              onChange={(event) => setOverwrite(event.target.checked)}
            />
            <span>Overwrite the output files</span>
          </label>
        </div>

        {problems.length > 0 && (
          <ul className="etl-problems">
            {problems.map((problem) => (
              <li key={problem}>{problem}</li>
            ))}
          </ul>
        )}

        <div className="field">
          <span className="field-label">Command</span>
          {commandError ? (
            <p className="text-dim">{commandError}</p>
          ) : (
            <pre className="etl-command" data-testid="etl-command">
              {command || 'The command appears once the dump is named.'}
            </pre>
          )}
          <div className="etl-actions">
            <button
              type="button"
              className="btn btn-sm"
              onClick={() => void copy()}
              disabled={!command}
              title="Copy the command to the clipboard"
            >
              <Copy size={14} />
              {copied ? 'Copied' : 'Copy'}
            </button>
          </div>
          <span className="field-hint">
            Copy it and run it in a terminal to do the same job outside the application.
          </span>
        </div>

        <div className="etl-actions">
          <button
            type="button"
            className="btn btn-sm btn-primary"
            onClick={() => startMutation.mutate()}
            disabled={running || startMutation.isPending || problems.length > 0}
          >
            <Play size={14} />
            Start
          </button>
          <button
            type="button"
            className="btn btn-sm"
            onClick={() => progress && cancelMutation.mutate(progress.id)}
            disabled={!running}
          >
            <Square size={14} />
            Cancel
          </button>
        </div>

        {progress && (
          <div className="etl-progress">
            <div className="param-row">
              <span>Status</span>
              <span className="text-dim">{statusLabel(progress.status)}</span>
            </div>
            <div className="param-row">
              <span>Lines</span>
              <span className="text-dim">{formatCount(progress.processed_lines)}</span>
            </div>
            <div className="param-row">
              <span>Items</span>
              <span className="text-dim">{formatCount(progress.matched_items)}</span>
            </div>
            <div className="param-row">
              <span>Properties</span>
              <span className="text-dim">{formatCount(progress.collected_properties)}</span>
            </div>
            <div className="param-row">
              <span>Speed</span>
              <span className="text-dim">{formatSpeed(progress.speed_mbs)}</span>
            </div>
            {progress.error && (
              <p className="etl-error">{progress.error}</p>
            )}
          </div>
        )}
      </div>
    </section>
  );
}
