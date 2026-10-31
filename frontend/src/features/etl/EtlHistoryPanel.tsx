/**
 * The history of the ETL runs.
 *
 * Every run the application started is recorded, and this panel shows
 * them. The record holds the command that was run, so a user can copy
 * it out of the history and run the same job again.
 *
 * The panel is read mostly: the command is the thing the user comes
 * for.
 */

import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Copy, Trash2 } from 'lucide-react';

import * as api from '../../bindings';
import type { EtlHistoryEntry } from '../../bindings';
import { writeClipboard } from '../console/ConsolePanel';
import { statusLabel } from './command';

/**
 * The props of the history panel.
 */
export interface EtlHistoryPanelProps {
  /** Called with a message to show to the user. */
  readonly onNotify?: (message: string, kind?: 'info' | 'error' | 'success') => void;
}

/**
 * The ETL history panel.
 */
export function EtlHistoryPanel({ onNotify }: EtlHistoryPanelProps) {
  const queryClient = useQueryClient();
  const [copied, setCopied] = useState<string | null>(null);

  const historyQuery = useQuery({
    queryKey: ['etl-history'],
    queryFn: () => api.listEtlHistory(50),
  });

  const deleteMutation = useMutation({
    mutationFn: (recordId: string) => api.deleteEtlHistoryEntry(recordId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['etl-history'] });
    },
    onError: (error: Error) => {
      onNotify?.(error.message, 'error');
    },
  });

  const copy = async (entry: EtlHistoryEntry) => {
    const ok = await writeClipboard(entry.full_command);
    if (!ok) {
      onNotify?.('The command could not be copied.', 'error');
      return;
    }
    setCopied(entry.id);
    window.setTimeout(() => setCopied(null), 1500);
  };

  const entries = historyQuery.data ?? [];

  return (
    <section className="panel">
      <div className="panel-header">
        <span>Job history</span>
        <span className="text-dim">{entries.length}</span>
      </div>

      <div className="panel-body">
        {entries.length === 0 ? (
          <span className="text-dim">No run yet.</span>
        ) : (
          entries.map((entry) => (
            <div className="etl-history-entry" key={entry.id}>
              <div className="param-row">
                <span className={`etl-status etl-status-${entry.status}`}>
                  {statusLabel(entry.status)}
                </span>
                <span className="text-dim">{entry.target_qids.join(', ')}</span>
                <span className="text-dim">{entry.started_at}</span>
              </div>

              <pre className="etl-command etl-command-small">{entry.full_command}</pre>

              {entry.error_message && <p className="etl-error">{entry.error_message}</p>}

              <div className="etl-actions">
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => void copy(entry)}
                  title="Copy the command to the clipboard"
                >
                  <Copy size={14} />
                  {copied === entry.id ? 'Copied' : 'Copy'}
                </button>
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={() => deleteMutation.mutate(entry.id)}
                  title="Remove the run from the history"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
          ))
        )}
      </div>
    </section>
  );
}
