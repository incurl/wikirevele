/**
 * The settings dialog.
 *
 * Holds the API credentials and the interface preferences, one tab
 * each: the connection tab carries the credentials, the zoom tab the
 * interface scale.
 *
 * The API key is write-only from the UI's point of view: the backend
 * never returns it, it is stored encrypted in PocketBase and only
 * decrypted by the Go process.
 */

import { useEffect, useState } from 'react';

import type { Settings } from '../../bindings';
import { applyZoom, clampZoom, MAX_ZOOM, MIN_ZOOM, ZOOM_PRESETS } from '../../ui/zoom';

interface SettingsDialogProps {
  /** Whether the dialog is shown. */
  open: boolean;
  /** The current settings, or null while they are loading. */
  settings: Settings | null;
  /** Closes the dialog. */
  onClose: () => void;
  /** Stores the credentials. */
  onSave: (apiKey: string, baseUrl: string) => void;
  /** Stores the interface preferences. */
  onSaveUI: (ui: { zoom: number; theme: string }) => void;
}

/**
 * The tabs of the settings dialog.
 */
export type SettingsTab = 'connection' | 'zoom';

/**
 * The tabs, in the order they are shown.
 */
export const SETTINGS_TABS: { value: SettingsTab; label: string }[] = [
  { value: 'connection', label: 'Connection' },
  { value: 'zoom', label: 'Zoom' },
];

/**
 * Formats a zoom factor for display.
 */
export function formatZoom(zoom: number): string {
  return `${clampZoom(zoom)}%`;
}

/**
 * The settings dialog.
 */
export function SettingsDialog({
  open,
  settings,
  onClose,
  onSave,
  onSaveUI,
}: SettingsDialogProps) {
  const [tab, setTab] = useState<SettingsTab>('connection');
  const [apiKey, setApiKey] = useState('');
  const [baseUrl, setBaseUrl] = useState('');
  const [zoom, setZoom] = useState(100);

  useEffect(() => {
    if (!settings) {
      return;
    }
    setBaseUrl(settings.base_url ?? '');
    setZoom(settings.ui.zoom);
  }, [settings]);

  if (!open) {
    return null;
  }

  const applyZoomLevel = (value: number) => {
    const factor = clampZoom(value);
    setZoom(factor);
    // The zoom is applied straight away, so that the reader sees the
    // effect before saving it.
    applyZoom(factor);
  };

  // Cancelling restores the persisted zoom, so that an unsaved change
  // does not outlive the dialog. The persisted value is read from the
  // settings rather than kept in a second piece of state, so that the
  // two cannot drift apart.
  const cancel = () => {
    const persisted = settings?.ui.zoom ?? 100;
    setZoom(persisted);
    applyZoom(persisted);
    onClose();
  };

  // The credentials and the zoom are stored together, so that a zoom
  // chosen on the other tab is not lost when the credentials are saved.
  const saveCredentials = () => {
    const persisted = settings?.ui.zoom ?? 100;
    if (zoom !== persisted) {
      onSaveUI({ zoom, theme: settings?.ui.theme ?? 'dark' });
    }
    onSave(apiKey.trim(), baseUrl.trim());
  };

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-label="Settings">
      <div className="modal">
        <div className="modal-header">
          <span>Settings</span>
          <button type="button" className="btn btn-sm" onClick={onClose} aria-label="Close">
            ×
          </button>
        </div>

        <div className="modal-tabs" role="tablist" aria-label="Settings sections">
          {SETTINGS_TABS.map((option) => (
            <button
              key={option.value}
              type="button"
              role="tab"
              id={`settings-tab-${option.value}`}
              aria-selected={tab === option.value}
              aria-controls={`settings-panel-${option.value}`}
              className={`modal-tab${tab === option.value ? ' modal-tab-active' : ''}`}
              onClick={() => setTab(option.value)}
            >
              {option.label}
            </button>
          ))}
        </div>

        <div
          className="modal-body"
          role="tabpanel"
          id={`settings-panel-${tab}`}
          aria-labelledby={`settings-tab-${tab}`}
        >
          {tab === 'connection' && (
            <>
              <div className="field">
                <label className="field-label" htmlFor="settings-api-key">
                  API key
                </label>
                <input
                  id="settings-api-key"
                  type="password"
                  placeholder={
                    settings?.has_api_key ? 'A key is stored. Leave empty to keep it.' : 'Paste the key'
                  }
                  value={apiKey}
                  onChange={(event) => setApiKey(event.target.value)}
                />
                <span className="field-hint">
                  {settings?.has_api_key
                    ? 'A key is stored. It is kept encrypted and is never sent back to this window.'
                    : 'No key is stored. The backend reads it, never the frontend.'}
                </span>
              </div>

              <div className="field">
                <label className="field-label" htmlFor="settings-base-url">
                  Base URL
                </label>
                <input
                  id="settings-base-url"
                  type="text"
                  placeholder="https://api.example.com"
                  value={baseUrl}
                  onChange={(event) => setBaseUrl(event.target.value)}
                />
              </div>
            </>
          )}

          {tab === 'zoom' && (
            <div className="field">
              <span className="field-label">Zoom</span>
              <div className="zoom-presets">
                {ZOOM_PRESETS.map((preset) => (
                  <button
                    key={preset}
                    type="button"
                    className={`btn btn-sm${zoom === preset ? ' btn-active' : ''}`}
                    onClick={() => applyZoomLevel(preset)}
                  >
                    {formatZoom(preset)}
                  </button>
                ))}
              </div>
              <input
                type="range"
                aria-label="Zoom"
                min={MIN_ZOOM}
                max={MAX_ZOOM}
                step={5}
                value={zoom}
                onChange={(event) => applyZoomLevel(Number(event.target.value))}
              />
              <span className="field-hint">
                The whole interface scales with this factor. It is applied straight away, and
                kept when you save.
              </span>
            </div>
          )}
        </div>

        <div className="modal-footer">
          <button type="button" className="btn" onClick={cancel}>
            Cancel
          </button>
          {tab === 'zoom' ? (
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => onSaveUI({ zoom, theme: settings?.ui.theme ?? 'dark' })}
            >
              Apply zoom
            </button>
          ) : (
            <button type="button" className="btn btn-primary" onClick={saveCredentials}>
              Save
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
