/**
 * The navigator of the application.
 *
 * It is the left rail, and it does one thing: it switches the view to a
 * scope. It holds no data panel of its own, so that the view can be
 * the whole width of the window.
 *
 * It collapses to its icons when the view needs the room. The names are
 * hidden rather than removed, so that a collapsed navigator is still a
 * navigator and not a row of guessed glyphs: every entry keeps its title
 * and its accessible name.
 */

import {
  Download,
  History,
  LayoutGrid,
  PanelLeftClose,
  PanelLeftOpen,
  Send,
  Upload,
  type LucideIcon,
} from "lucide-react";

import { SCOPES, type ScopeId } from "./scopes";

/**
 * The icon of each scope.
 */
const ICONS: Record<ScopeId, LucideIcon> = {
  workarea: LayoutGrid,
  extract: Download,
  import: Upload,
  submit: Send,
  history: History,
};

/**
 * The props of the navigator.
 */
export interface NavigatorProps {
  /** The scope the view shows. */
  readonly active: ScopeId;
  /** Called when the user picks a scope. */
  readonly onSelect: (scope: ScopeId) => void;
  /** Whether the navigator is collapsed to its icons. */
  readonly collapsed: boolean;
  /** Called when the user collapses or expands the navigator. */
  readonly onToggle: () => void;
}

/**
 * The navigator.
 */
export function Navigator({ active, onSelect, collapsed, onToggle }: NavigatorProps) {
  const ToggleIcon = collapsed ? PanelLeftOpen : PanelLeftClose;
  const listId = "navigator-scopes";

  return (
    <nav
      className="navigator"
      data-collapsed={collapsed ? "true" : "false"}
      aria-label="Scopes"
    >
      {/*
        The toggle is in a header and not at the foot of the list: it is
        the first thing to reach for, and a control below five entries
        reads as part of the list rather than as the control of the
        rail. The jobs rail has the same header, so the two are
        collapsed the same way.
      */}
      <div className="navigator-header">
        <span className="navigator-title">Scopes</span>

        <button
          type="button"
          className="navigator-toggle"
          onClick={onToggle}
          aria-expanded={!collapsed}
          aria-controls={listId}
          title={collapsed ? "Expand the navigator" : "Collapse the navigator"}
        >
          <ToggleIcon className="navigator-icon" aria-hidden="true" />
          <span className="navigator-label">
            <span className="navigator-name">{collapsed ? "Expand" : "Collapse"}</span>
          </span>
        </button>
      </div>

      <ul className="navigator-list" id={listId}>
        {SCOPES.map((scope) => {
          const Icon = ICONS[scope.id];
          const current = scope.id === active;

          return (
            <li key={scope.id}>
              <button
                type="button"
                className="navigator-item"
                data-active={current ? "true" : "false"}
                onClick={() => onSelect(scope.id)}
                aria-current={current ? "page" : undefined}
                title={collapsed ? `${scope.label} — ${scope.hint}` : scope.hint}
              >
                <Icon className="navigator-icon" aria-hidden="true" />
                <span className="navigator-label">
                  <span className="navigator-name">{scope.label}</span>
                  <span className="navigator-hint">{scope.hint}</span>
                </span>
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
