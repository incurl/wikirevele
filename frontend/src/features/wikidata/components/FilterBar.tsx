/**
 * The filter bar of the panel.
 *
 * The filters are the ones the Paris exploration needs: a property the
 * item must carry, the place it is linked to, its genre and its
 * occupation. All four are the same shape in the data: a claim whose
 * value is an entity, so they share one filter mechanism.
 *
 * The bar writes Refine filters, and the provider applies them.
 */

import { Filter, X } from "lucide-react";

import type { CrudFilter } from "@refinedev/core";

import { filterField, filterValue } from "../provider";

/**
 * The identifier of Paris.
 */
export const PARIS_QID = "Q90";

/**
 * One filter of the bar.
 */
export interface FilterDefinition {
  /** The identifier of the filter, used by the panel. */
  readonly id: string;
  /** The name shown on the control. */
  readonly label: string;
  /** The property the filter is about, such as "P131". */
  readonly propertyId: string;
  /** The kind of the filter. */
  readonly kind: "entity" | "text";
  /** The values that can be chosen. */
  readonly options?: readonly { value: string; label: string }[];
}

/**
 * The filters the panel offers.
 *
 * The values are the identifiers, because the subset carries no labels
 * for the entities it points at. The label of a value is used when the
 * panel knows one.
 */
export const FILTERS: readonly FilterDefinition[] = [
  {
    id: "paris",
    label: "Located in",
    propertyId: "P131",
    kind: "entity",
    options: [{ value: PARIS_QID, label: "Paris (Q90)" }],
  },
  {
    id: "genre",
    label: "Genre",
    propertyId: "P136",
    kind: "entity",
  },
  {
    id: "occupation",
    label: "Occupation",
    propertyId: "P106",
    kind: "entity",
  },
  {
    id: "property",
    label: "Has property",
    propertyId: "P31",
    kind: "entity",
  },
];

/**
 * The props of the filter bar.
 */
export interface FilterBarProps {
  /** The filters that are applied. */
  readonly filters: readonly CrudFilter[];
  /** Called when a filter is added or removed. */
  readonly onChange: (filters: CrudFilter[]) => void;
  /** The number of items that match the filters. */
  readonly total?: number;
}

/**
 * The filter bar.
 */
export function FilterBar({ filters, onChange, total }: FilterBarProps) {
  /**
   * Whether a filter of the definition is applied.
   */
  const isApplied = (definition: FilterDefinition): boolean =>
    filters.some((filter) => filterField(filter) === `claims.${definition.propertyId}`);

  /**
   * Adds a filter, or removes it when it is already there.
   */
  const toggle = (definition: FilterDefinition, value: string) => {
    const field = `claims.${definition.propertyId}`;

    if (isApplied(definition)) {
      onChange(filters.filter((filter) => filterField(filter) !== field));
      return;
    }

    onChange([...filters, { field, operator: "eq", value }]);
  };

  /**
   * The value a filter is set to, when it is applied.
   */
  const valueOf = (definition: FilterDefinition): string | null => {
    for (const filter of filters) {
      if (filterField(filter) === `claims.${definition.propertyId}`) {
        return String(filterValue(filter));
      }
    }
    return null;
  };

  const active = filters.filter((filter) => filterField(filter) !== null);

  return (
    <div className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card p-2">
      <span className="inline-flex items-center gap-1 px-1 text-xs font-medium text-muted-foreground">
        <Filter className="h-3.5 w-3.5" aria-hidden="true" />
        Filters
      </span>

      {FILTERS.map((definition) => {
        const value = valueOf(definition);

        if (definition.options && definition.options.length > 0) {
          return (
            <button
              key={definition.id}
              type="button"
              onClick={() => toggle(definition, definition.options![0].value)}
              className={
                value
                  ? "rounded-md bg-primary px-2 py-1 text-xs text-primary-foreground"
                  : "rounded-md border border-border px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
              }
              aria-pressed={value !== null}
            >
              {definition.label}
              {value ? `: ${value}` : ""}
            </button>
          );
        }

        // A filter without a fixed list of values takes the value from
        // the user, as an identifier.
        return (
          <label key={definition.id} className="inline-flex items-center gap-1 text-xs">
            <span className="text-muted-foreground">{definition.label}</span>
            <input
              className="h-7 w-28 rounded-md border border-border bg-background px-2 font-mono text-xs outline-none focus:border-ring"
              placeholder={definition.propertyId}
              defaultValue={value ?? ""}
              onKeyDown={(event) => {
                if (event.key !== "Enter") {
                  return;
                }
                const input = event.currentTarget;
                const typed = input.value.trim();
                if (!typed) {
                  return;
                }
                toggle(definition, typed);
                input.value = "";
              }}
            />
          </label>
        );
      })}

      {active.length > 0 ? (
        <button
          type="button"
          className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
          onClick={() => onChange([])}
        >
          <X className="h-3 w-3" aria-hidden="true" />
          Clear
        </button>
      ) : null}

      {total !== undefined ? (
        <span className="ml-auto px-1 text-xs text-muted-foreground">{total} items</span>
      ) : null}
    </div>
  );
}
