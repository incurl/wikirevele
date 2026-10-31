/**
 * The import panel of the Wikidata dataset.
 *
 * The panel starts an import of the NDJSON files the ETL worker
 * writes, and shows how it goes. The import runs in the backend, so
 * the window stays responsive while a large file is being imported.
 *
 * The progress is pushed as a Wails event, so the panel does not have
 * to poll: the bar moves as the records are written.
 */

import { useCallback, useEffect, useState } from "react";
import { useInvalidate } from "@refinedev/core";
import { useQueryClient } from "@tanstack/react-query";
import { FolderOpen, Loader2, Upload } from "lucide-react";
import { Events } from "@wailsio/runtime";

import * as api from "../../../bindings";
import type { WikidataPackage } from "../../../bindings";
import { RESOURCE } from "../provider";
import { DATASET_QUERY_KEY } from "../backend";

/**
 * The event the backend pushes the progress on.
 */
const IMPORT_PROGRESS_EVENT = "wikidata:import:progress";

/**
 * The props of the import panel.
 */
export interface ImportPanelProps {
  /** Called when the import has written new records. */
  readonly onImported?: () => void;
}

/**
 * The import panel.
 */
export function ImportPanel({ onImported }: ImportPanelProps) {
  // The list of the items is a Refine query, so it is invalidated with
  // the hook of Refine: the import has written records, and the list
  // has to be read again.
  const invalidate = useInvalidate();
  const queryClient = useQueryClient();
  const [progress, setProgress] = useState<WikidataPackage.ImportProgress | null>(null);
  const [path, setPath] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // The progress of the import that ran before the panel was opened
  // is asked for, so that the panel shows the outcome of it.
  useEffect(() => {
    let cancelled = false;

    api
      .getWikidataImportProgress()
      .then((state) => {
        if (!cancelled && state) {
          setProgress(state);
        }
      })
      .catch(() => {
        // The progress is not known: the panel shows nothing until
        // the next import pushes one.
      });

    return () => {
      cancelled = true;
    };
  }, []);

  // The path of the dataset is the data directory of the application:
  // the ETL worker leaves its output there, and the user does not have
  // to type it.
  useEffect(() => {
    let cancelled = false;

    api
      .getWikidataDataDir()
      .then((dir) => {
        if (!cancelled && dir) {
          setPath((current) => current || dir);
        }
      })
      .catch(() => {
        // The directory is not known: the user types the path.
      });

    return () => {
      cancelled = true;
    };
  }, []);

  // The progress of a running import is pushed as an event.
  useEffect(() => {
    const unsubscribe = Events.On(IMPORT_PROGRESS_EVENT, (event) => {
      const payload = event.data as WikidataPackage.ImportProgress | undefined;
      if (!payload) {
        return;
      }
      setProgress(payload);

      // The records are in the database now: the list has to be read
      // again.
      if (payload.done) {
        // The list of the items is a Refine query, and the dataset the
        // panel reads is a query of its own. Both have to be read
        // again: the import has written records.
        void invalidate({ resource: RESOURCE, invalidates: ["list", "detail", "many"] });
        void queryClient.invalidateQueries({ queryKey: DATASET_QUERY_KEY });
        onImported?.();
      }
    });

    return () => {
      unsubscribe();
    };
  }, [invalidate, queryClient, onImported]);

  const start = useCallback(
    async (kind: "items" | "properties") => {
      if (!path) {
        setError("The path of the file is required.");
        return;
      }

      setBusy(true);
      setError(null);

      try {
        await api.importWikidataFile(kind, path);
      } catch (cause) {
        setError(cause instanceof Error ? cause.message : String(cause));
      } finally {
        setBusy(false);
      }
    },
    [path],
  );

  const importDirectory = useCallback(async () => {
    if (!path) {
      setError("The path of the directory is required.");
      return;
    }

    setBusy(true);
    setError(null);

    try {
      await api.importWikidataDirectory(path);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      setBusy(false);
    }
  }, [path]);

  const cancel = useCallback(async () => {
    try {
      await api.cancelWikidataImport();
    } catch {
      // The import is already over.
    }
  }, []);

  // An import is running when the panel started one, or when the
  // backend says that one is running.
  const running = busy || (progress !== null && (progress.running || !progress.done));

  return (
    <div className="rounded-lg border border-border bg-card p-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground">
          <Upload className="h-3.5 w-3.5" aria-hidden="true" />
          Import the dataset
        </span>

        <input
          className="h-7 min-w-48 flex-1 rounded-md border border-border bg-background px-2 font-mono text-xs outline-none focus:border-ring"
          placeholder="the path of the NDJSON file or of the directory"
          value={path}
          onChange={(event) => setPath(event.target.value)}
          aria-label="The path of the dataset"
        />

        <button
          type="button"
          className="inline-flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground disabled:opacity-50"
          onClick={() => void start("items")}
          disabled={running}
        >
          Items
        </button>

        <button
          type="button"
          className="inline-flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground disabled:opacity-50"
          onClick={() => void start("properties")}
          disabled={running}
        >
          Properties
        </button>

        <button
          type="button"
          className="inline-flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground disabled:opacity-50"
          onClick={() => void importDirectory()}
          disabled={running}
        >
          <FolderOpen className="h-3 w-3" aria-hidden="true" />
          Directory
        </button>

        {running ? (
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
            onClick={() => void cancel()}
          >
            <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
            Stop
          </button>
        ) : null}
      </div>

      {error ? <p className="mt-2 text-xs text-destructive">{error}</p> : null}

      {progress && (progress.read > 0 || progress.done || progress.running) ? (
        <ImportProgressView progress={progress} />
      ) : null}
    </div>
  );
}

/**
 * The progress of an import.
 */
function ImportProgressView({ progress }: { readonly progress: WikidataPackage.ImportProgress }) {
  const total = (progress.read ?? 0) + (progress.skipped ?? 0);
  const percent = total > 0 ? Math.round(((progress.imported ?? 0) + (progress.updated ?? 0)) / total * 100) : 0;

  return (
    <div className="mt-2 flex flex-col gap-1">
      <div className="flex items-center gap-2 text-xs text-muted-foreground">
        <span className="font-mono">{progress.file ?? ""}</span>
        <span>·</span>
        <span>
          {progress.imported ?? 0} imported
        </span>
        <span>·</span>
        <span>
          {progress.updated ?? 0} updated
        </span>
        {progress.skipped ? (
          <>
            <span>·</span>
            <span>
              {progress.skipped} skipped
            </span>
          </>
        ) : null}
        {progress.done ? (
          <>
            <span>·</span>
            <span className="text-foreground">done</span>
          </>
        ) : (
          <>
            <span>·</span>
            <span>running</span>
          </>
        )}
      </div>

      <div
        className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-valuenow={percent}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className="h-full rounded-full bg-primary transition-[width]"
          style={{ width: `${progress.done ? 100 : percent}%` }}
        />
      </div>

      {progress.error ? (
        <p className="text-xs text-destructive">{progress.error}</p>
      ) : null}
    </div>
  );
}
