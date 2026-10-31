/**
 * The Wikidata panel.
 *
 * It is a Refine application of its own: a data provider over the
 * source, a resource, and the two pages. It is mounted inside the
 * application shell, so the panel does not have to know about the
 * rest of the application.
 *
 * The panel is read-only. The data provider refuses the mutations,
 * and the resource declares no create and no edit page, so the
 * interface never offers them.
 *
 * The panel works on the records of the database, where the importer
 * put them. It never reads a dump and never parses an NDJSON file.
 */

import { useMemo } from "react";
import { Refine } from "@refinedev/core";
import routerProvider from "@refinedev/react-router";
import { Route, Routes } from "react-router";

import { ListView, ListViewHeader } from "@/components/refine-ui/views/list-view";
import { ShowView, ShowViewHeader } from "@/components/refine-ui/views/show-view";

import { WikidataItemList } from "./pages/WikidataItemList";
import { WikidataItemShow } from "./pages/WikidataItemShow";
import { createWikidataProvider, RESOURCE } from "./provider";
import { wikidataItemResource } from "./resources";
import { useWikidataDataset } from "./backend";
import { EMPTY_SOURCE } from "./source";
import type { WikidataSource } from "./source";

/**
 * The props of the panel.
 */
export interface WikidataPanelProps {
  /**
   * Where the items come from.
   *
   * When it is not given, the panel reads the imported records from
   * the database, which is the normal case.
   */
  readonly source?: WikidataSource;
  /** Whether the graph is shown in the cards. */
  readonly showGraph?: boolean;
}

/**
 * The Wikidata panel.
 */
export function WikidataPanel({ source: givenSource, showGraph = false }: WikidataPanelProps) {
  const { source: readSource, loading, error } = useWikidataDataset();

  // A source that is given wins over the one that is read, so that a
  // caller can browse a subset of its own.
  const source = givenSource ?? readSource;

  const provider = useMemo(
    () => createWikidataProvider(source ?? EMPTY_SOURCE),
    [source],
  );

  if (error) {
    return (
      <div className="wikidata-panel">
        <section className="panel">
          <div className="panel-header">
            <span>Wikidata items</span>
          </div>
          <div className="panel-body">
            <p className="text-dim">
              The dataset could not be read: {error.message}
            </p>
          </div>
        </section>
      </div>
    );
  }

  return (
    <Refine dataProvider={provider} routerProvider={routerProvider} resources={[wikidataItemResource]}>
      {/*
        The theme is scoped to the panel, so that it does not change the
        colours of the application around it.
      */}
      <div className="wikidata-panel">
        {/*
          The two pages are routes, not one after the other: the list
          is at / and the item is at /wikidata/:id. Without the
          routes both pages are rendered at once.

          The list is the landing page, so it answers the root path
          and the /wikidata path both. The application starts at the
          root, and a panel that only answers /wikidata would be one
          the user cannot reach: there is no link to it.
        */}
        <Routes>
          <Route
            path="/"
            element={
              <ListView>
                <ListViewHeader title="Wikidata items" canCreate={false} />
                {loading ? (
                  <p className="text-sm text-muted-foreground">Loading the dataset…</p>
                ) : (
                  <WikidataItemList showGraph={showGraph} />
                )}
              </ListView>
            }
          />
          <Route
            path="/wikidata"
            element={
              <ListView>
                <ListViewHeader title="Wikidata items" canCreate={false} />
                {loading ? (
                  <p className="text-sm text-muted-foreground">Loading the dataset…</p>
                ) : (
                  <WikidataItemList showGraph={showGraph} />
                )}
              </ListView>
            }
          />
          <Route
            path="/wikidata/:id"
            element={
              <ShowView>
                <ShowViewHeader title="Wikidata item" />
                <WikidataItemShow />
              </ShowView>
            }
          />
        </Routes>
      </div>
    </Refine>
  );
}

/**
 * The name of the resource, re-exported for the tests.
 */
export { RESOURCE };
