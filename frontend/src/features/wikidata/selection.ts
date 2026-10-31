/**
 * The selection of the workarea.
 *
 * The workarea is a grid of cards, and the user manipulates the items
 * through it: they select one, they select several, and they act on
 * what is selected.
 *
 * The selection is kept out of the components: the rules of it are
 * plain, and they are worth testing on their own.
 */

/**
 * The selection of a set of items.
 */
export interface Selection {
  /** The identifiers of the selected items. */
  readonly ids: ReadonlySet<string>;
}

/**
 * An empty selection.
 */
export const EMPTY_SELECTION: Selection = { ids: new Set<string>() };

/**
 * Tells whether an item is selected.
 */
export function isSelected(selection: Selection, id: string): boolean {
  return selection.ids.has(id);
}

/**
 * Toggles one item.
 *
 * The item is added when it is not selected, and removed when it is.
 */
export function toggle(selection: Selection, id: string): Selection {
  const ids = new Set(selection.ids);

  if (ids.has(id)) {
    ids.delete(id);
  } else {
    ids.add(id);
  }

  return { ids };
}

/**
 * Selects every item of the list.
 *
 * The items that are already selected are kept, so that a selection
 * that spans two pages survives.
 */
export function selectAll(selection: Selection, ids: readonly string[]): Selection {
  const next = new Set(selection.ids);

  for (const id of ids) {
    next.add(id);
  }

  return { ids: next };
}

/**
 * Clears the selection.
 */
export function clear(): Selection {
  return EMPTY_SELECTION;
}

/**
 * Tells whether a modifier is held down.
 *
 * The command key on Apple platforms, the control key elsewhere. It
 * is the modifier that turns a click into a toggle of one item among
 * many.
 */
export function isMultiSelect(event: {
  metaKey: boolean;
  ctrlKey: boolean;
}): boolean {
  return event.metaKey || event.ctrlKey;
}

/**
 * The summary of a selection, for the interface.
 *
 * It says how many items are selected, and it says when every item of
 * the list is selected, so that the reader knows whether the selection
 * is the whole dataset or only the page.
 */
export function summarize(
  selection: Selection,
  visibleCount: number,
  totalCount: number,
): string {
  const count = selection.ids.size;

  if (count === 0) {
    return totalCount > 0 ? `${totalCount} items` : 'No item';
  }

  if (visibleCount > 0 && count >= visibleCount && totalCount > visibleCount) {
    return `${count} selected (the whole page)`;
  }

  return `${count} selected`;
}
