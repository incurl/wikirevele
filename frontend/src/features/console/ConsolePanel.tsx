/**
 * The debug console.
 *
 * A log view at the bottom of the window that records what the
 * application and the remote APIs did: the requests that were sent, the
 * responses that came back and the errors that were raised.
 *
 * It is deliberately a plain text view rather than a rich editor: the
 * point is to read a raw API response verbatim.
 */

import { useCallback, useLayoutEffect, useRef, useState } from 'react';

import type { ConsoleEntry } from '../../bindings';

interface ConsolePanelProps {
  /** The recorded entries, oldest first. */
  entries: ConsoleEntry[];
  /** Discards the recorded entries. */
  onClear: () => void;
}

type LevelFilter = 'all' | 'error' | 'raw';

/** The levels that the filter can show. */
const LEVEL_FILTERS: { value: LevelFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'error', label: 'Errors' },
  { value: 'raw', label: 'Raw' },
];

/**
 * Decides whether an entry passes the level filter.
 */
export function matchesFilter(entry: ConsoleEntry, filter: LevelFilter): boolean {
  switch (filter) {
    case 'error':
      return entry.level === 'error' || entry.level === 'warn';
    case 'raw':
      return entry.level === 'raw';
    default:
      return true;
  }
}

/**
 * Counts the entries that report a problem.
 *
 * The errors and the warnings are counted together, because both mean
 * that something needs attention.
 */
export function countErrors(entries: ConsoleEntry[]): number {
  return entries.filter((entry) => entry.level === 'error' || entry.level === 'warn').length;
}

/**
 * Formats the entry detail for display.
 *
 * A raw API body is pretty printed when it is valid JSON, so that it can
 * be read, and shown verbatim otherwise.
 */
export function formatDetail(detail: string): string {
  const trimmed = detail.trim();
  if (!trimmed) {
    return '';
  }
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return detail;
  }
}

/**
 * Renders one entry as plain text, for the clipboard.
 */
export function entryToText(entry: ConsoleEntry): string {
  const head = `${entry.time} [${entry.level}] ${entry.source}: ${entry.message}`;
  const detail = formatDetail(entry.detail);
  return detail ? `${head}\n${detail}` : head;
}

/**
 * Renders several entries as plain text, one per block.
 */
export function entriesToText(entries: ConsoleEntry[]): string {
  return entries.map(entryToText).join('\n');
}

/**
 * Writes text to the clipboard.
 *
 * navigator.clipboard needs a secure context and a user gesture, which a
 * webview does not always provide. When it is unavailable the legacy
 * copy command is used instead: without it the copy buttons would
 * silently do nothing in the native window.
 *
 * Reports whether the text reached the clipboard.
 */
export async function writeClipboard(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // Fall through to the legacy path.
  }

  try {
    const area = document.createElement('textarea');
    area.value = text;
    area.setAttribute('readonly', '');
    // Keep it out of sight without using display:none, which would make
    // it unselectable and break the copy command.
    area.style.position = 'fixed';
    area.style.top = '-1000px';
    area.style.opacity = '0';
    document.body.appendChild(area);
    area.select();
    const copied = document.execCommand('copy');
    document.body.removeChild(area);
    return copied;
  } catch {
    return false;
  }
}

/**
 * Reports whether the user is dragging over text rather than clicking.
 *
 * Selecting a message used to also toggle the detail view, because the
 * click that ends a drag fired the row handler. The selection is checked
 * so that reading the log and copying from it stay out of each other's
 * way.
 */
export function hasTextSelection(): boolean {
  const selection = window.getSelection();
  return selection !== null && selection.toString().trim() !== '';
}

/**
 * The debug console panel.
 */
export function ConsolePanel({ entries, onClear }: ConsolePanelProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [filter, setFilter] = useState<LevelFilter>('all');
  const [follow, setFollow] = useState(true);
  const [selected, setSelected] = useState<number | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  const bodyRef = useRef<HTMLDivElement>(null);

  const visible = entries.filter((entry) => matchesFilter(entry, filter));
  const active = visible.find((entry) => entry.id === selected) ?? null;

  const errorCount = countErrors(entries);

  // The view follows the newest entry, unless the reader scrolled away.
  useLayoutEffect(() => {
    if (!follow || !bodyRef.current) {
      return;
    }
    bodyRef.current.scrollTop = bodyRef.current.scrollHeight;
  }, [visible.length, follow]);

  const copy = useCallback(async (key: string, text: string) => {
    const ok = await writeClipboard(text);
    if (!ok) {
      return;
    }
    setCopied(key);
    window.setTimeout(() => setCopied(null), 1500);
  }, []);

  const toggleEntry = (id: number) => {
    // A drag that ends in a click must not toggle the detail view.
    if (hasTextSelection()) {
      return;
    }
    setSelected((current) => (current === id ? null : id));
  };

  return (
    <section className="console">
      <div className="console-header">
        <button
          type="button"
          className="btn btn-sm"
          onClick={() => setCollapsed((value) => !value)}
          title={collapsed ? 'Expand the console' : 'Collapse the console'}
        >
          {collapsed ? '▲' : '▼'}
        </button>

        <span className="console-summary">
          Console
          {errorCount > 0 && (
            <span className="console-error-count">
              {' · '}
              {errorCount} error{errorCount === 1 ? '' : 's'}
            </span>
          )}
        </span>

        <span className="grow" />

        {LEVEL_FILTERS.map((option) => (
          <button
            key={option.value}
            type="button"
            className={`btn btn-sm${filter === option.value ? ' btn-active' : ''}`}
            onClick={() => setFilter(option.value)}
          >
            {option.label}
          </button>
        ))}

        <button
          type="button"
          className={`btn btn-sm${follow ? ' btn-active' : ''}`}
          onClick={() => setFollow((value) => !value)}
          title="Scroll to the newest entry"
        >
          Follow
        </button>

        <button
          type="button"
          className="btn btn-sm"
          onClick={() => void copy('all', entriesToText(visible))}
          title="Copy every visible entry"
        >
          {copied === 'all' ? 'Copied' : 'Copy'}
        </button>

        <button
          type="button"
          className="btn btn-sm"
          onClick={() => {
            setSelected(null);
            onClear();
          }}
        >
          Clear
        </button>
      </div>

      {!collapsed && (
        <div className="console-body" ref={bodyRef}>
          {visible.length === 0 ? (
            <div className="console-empty">
              No console output yet. API requests and errors appear here.
            </div>
          ) : (
            visible.map((entry) => (
              <div
                key={entry.id}
                className={`console-entry console-level-${entry.level}${
                  selected === entry.id ? ' console-entry-selected' : ''
                }`}
                onClick={() => toggleEntry(entry.id)}
                role="button"
                tabIndex={0}
                onKeyDown={(event) => {
                  if (event.key === 'Enter' || event.key === ' ') {
                    event.preventDefault();
                    toggleEntry(entry.id);
                  }
                }}
              >
                <span className="console-time">{entry.time}</span>
                <span className="console-level">{entry.level}</span>
                <span className="console-source">{entry.source}</span>
                <span className="console-message">{entry.message}</span>
                <button
                  type="button"
                  className="console-entry-copy"
                  title="Copy this entry"
                  aria-label={`Copy the ${entry.level} entry from ${entry.source}`}
                  onClick={(event) => {
                    // The row toggles the detail view; the copy button
                    // must not.
                    event.stopPropagation();
                    void copy(`entry:${entry.id}`, entryToText(entry));
                  }}
                >
                  {copied === `entry:${entry.id}` ? 'Copied' : 'Copy'}
                </button>
              </div>
            ))
          )}
        </div>
      )}

      {!collapsed && active?.detail && (
        <div className="console-detail">
          <div className="console-detail-head">
            <span>
              {active.time} · {active.source}
            </span>
            <span className="grow" />
            <span className="console-detail-hint">Select the text to copy it</span>
            <button
              type="button"
              className="btn btn-sm"
              onClick={() => void copy(`detail:${active.id}`, formatDetail(active.detail))}
            >
              {copied === `detail:${active.id}` ? 'Copied' : 'Copy'}
            </button>
          </div>
          <pre className="console-detail-body">{formatDetail(active.detail)}</pre>
        </div>
      )}
    </section>
  );
}
