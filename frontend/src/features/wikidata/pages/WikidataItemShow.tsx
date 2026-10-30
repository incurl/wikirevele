/**
 * The detail page of an item.
 *
 * It shows the whole item: every property, with the qualifiers and the
 * references of every statement, and the graph of its neighbourhood.
 */

import { useMemo } from "react";
import { useShow } from "@refinedev/core";
import { Loader2 } from "lucide-react";

import { WikidataEntityCard } from "../components/WikidataEntityCard";
import { RESOURCE, viewOf } from "../provider";
import { placesOf } from "../backend";

/**
 * The detail page.
 */
export function WikidataItemShow() {
  const { result, query } = useShow({
    resource: RESOURCE,
  });

  const view = useMemo(() => viewOf(result), [result]);
  const places = useMemo(() => (view ? placesOf(view) : []), [view]);

  if (query.isLoading) {
    return (
      <div className="flex items-center justify-center gap-2 p-8 text-sm text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        Loading the item…
      </div>
    );
  }

  if (query.isError || !view) {
    return (
      <div className="rounded-lg border border-destructive/40 bg-destructive/10 p-4 text-sm">
        <p className="font-medium">The item could not be loaded.</p>
        <p className="mt-1 text-muted-foreground">{query.error?.message}</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      <WikidataEntityCard
        item={view}
        places={places}
        defaultExpanded
        showGraph
      />
    </div>
  );
}
