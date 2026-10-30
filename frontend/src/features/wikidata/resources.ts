/**
 * The Refine resource of the panel.
 *
 * The panel is a browse panel: it shows the items and it does not
 * change them. The resource therefore declares a list and a show page
 * and nothing else, so that the Refine components do not offer to
 * create or to edit an item.
 */

import type { ResourceProps } from "@refinedev/core";

import { RESOURCE } from "./provider";

/**
 * The resource of the Wikidata items.
 *
 * The label is what the shadcn headers show as the page title.
 */
export const wikidataItemResource: ResourceProps = {
  name: RESOURCE,
  list: "/wikidata",
  show: "/wikidata/:id",
  meta: {
    label: "Wikidata items",
    // The panel is read-only: the items are the dump's, and the dump
    // is the source of truth. There is no create page and no edit
    // page, so the buttons of the headers do not appear.
    canDelete: false,
  },
};
