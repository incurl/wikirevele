/**
 * The scopes of the application.
 *
 * The window has one navigator and one view. A scope is a pane that the
 * navigator can switch the view to; the navigator itself holds no data
 * pane, only the list of them.
 *
 * The list is kept here, out of the components, because the navigation
 * and the view have to agree on the same order and the same names.
 */

/**
 * The identifier of a scope.
 */
export type ScopeId = "workarea" | "extract" | "import" | "submit" | "history" | "sync";

/**
 * One scope of the navigator.
 */
export interface Scope {
  /** The identifier of the scope. */
  readonly id: ScopeId;
  /** The name shown in the navigator. */
  readonly label: string;
  /** The one-line description, shown under the name when the navigator is open. */
  readonly hint: string;
}

/**
 * The scopes, in the order of the navigator.
 *
 * The workarea is first and it is the default: it is where the data is,
 * and the other scopes are the tools that put data there.
 */
export const SCOPES: readonly Scope[] = [
  { id: "workarea", label: "Workarea", hint: "The Wikidata cards" },
  { id: "extract", label: "Extract", hint: "Filter a dump into datasets" },
  { id: "import", label: "Import", hint: "Load the datasets into the database" },
  { id: "submit", label: "Submit", hint: "Send a job to the queue" },
  { id: "history", label: "History", hint: "The runs of the extraction" },
  { id: "sync", label: "Sync", hint: "Move the collections to a server" },
];

/**
 * The scope the application starts on.
 */
export const DEFAULT_SCOPE: ScopeId = "workarea";

/**
 * Reads the scope a path belongs to.
 *
 * The item pages are routes of their own, and they belong to the
 * workarea: a link to an item has to switch the view to the cards, or
 * the page would be addressed and not shown.
 *
 * A path that is not one of them keeps the scope the navigator has.
 */
export function scopeOfPath(pathname: string): ScopeId | null {
  if (pathname === "/" || pathname === "/wikidata" || pathname.startsWith("/wikidata/")) {
    return "workarea";
  }

  return null;
}

/**
 * Tells whether a scope is one of the scopes.
 */
export function isScope(value: string): value is ScopeId {
  return SCOPES.some((scope) => scope.id === value);
}
