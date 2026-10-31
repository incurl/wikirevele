/**
 * A collapsible panel of the tool rails.
 *
 * The rails hold the tools of the application: the extraction and the
 * import on the left, the history and the connection on the right. The
 * canvas is the workarea, and it is the one that must stay wide, so
 * the rails have to be able to give their room back.
 *
 * The panel keeps its state when it is closed, so that a form the user
 * filled in is still there when they open it again.
 */

import { useState, type ReactNode } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '../../components/ui/collapsible';

/**
 * The props of a tool panel.
 */
export interface ToolPanelProps {
  /** The name of the tool, shown in the header. */
  readonly title: string;
  /** What the panel holds when it is open. */
  readonly children: ReactNode;
  /** Whether the panel starts open. */
  readonly defaultOpen?: boolean;
  /** A short line under the title, shown when the panel is open. */
  readonly hint?: string;
  /** A count shown on the right of the header, such as the number of runs. */
  readonly badge?: string | number;
}

/**
 * A collapsible panel of a tool rail.
 */
export function ToolPanel({
  title,
  children,
  defaultOpen = false,
  hint,
  badge,
}: ToolPanelProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <Collapsible
      open={open}
      onOpenChange={setOpen}
      className="tool-panel"
      data-open={open ? 'true' : 'false'}
    >
      <CollapsibleTrigger asChild>
        <button type="button" className="tool-panel-header" aria-expanded={open}>
          <span className="tool-panel-chevron" aria-hidden="true">
            {open ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronRight className="h-3.5 w-3.5" />}
          </span>
          <span className="tool-panel-title">{title}</span>
          {badge !== undefined && badge !== '' ? (
            <span className="tool-panel-badge">{badge}</span>
          ) : null}
        </button>
      </CollapsibleTrigger>

      <CollapsibleContent className="tool-panel-body">
        {hint ? <p className="tool-panel-hint">{hint}</p> : null}
        {children}
      </CollapsibleContent>
    </Collapsible>
  );
}
