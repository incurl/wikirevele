/**
 * The card of a Wikidata item.
 *
 * The card has two levels: a compact preview, which is what the grid
 * shows, and a detail panel, which opens under the preview. Both read
 * the same view model, so they cannot disagree.
 *
 * The card never changes the item it was given: it reads the view
 * model, which is built from the dump and is not modified.
 */

import { useState, type MouseEvent } from "react";
import { ChevronDown, ChevronRight, ExternalLink } from "lucide-react";
import { Link } from "react-router";

import type { EntityView } from "../view";
import { entityUrl } from "../view";
import { ValueRenderer } from "../fields/ValueRenderer";
import { EntityGraph } from "./EntityGraph";
import { isMultiSelect } from "../selection";

/**
 * The props of the card.
 */
export interface WikidataEntityCardProps {
  /** The item to show. */
  readonly item: EntityView;
  /** The links the extractor found for the item. */
  readonly places?: readonly { placeQid: string; propertyKind: "filming" | "narrative" }[];
  /** Whether the card opens with its detail panel open. */
  readonly defaultExpanded?: boolean;
  /** Called when the card is opened or closed. */
  readonly onToggle?: (expanded: boolean) => void;
  /** Whether the graph of the neighbourhood is shown. */
  readonly showGraph?: boolean;
  /** Whether the item is selected in the workarea. */
  readonly selected?: boolean;
  /** Called when the user selects the item, or clears the selection. */
  readonly onSelect?: (id: string, additive: boolean) => void;
}

/**
 * The card of a Wikidata item.
 */
export function WikidataEntityCard({
  item,
  places = [],
  defaultExpanded = false,
  onToggle,
  showGraph = false,
  selected = false,
  onSelect,
}: WikidataEntityCardProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);

  const toggle = () => {
    const next = !expanded;
    setExpanded(next);
    onToggle?.(next);
  };

  // The click on the card selects the item. The modifier turns it
  // into a toggle of one item among many.
  const select = (event: MouseEvent) => {
    if (!onSelect) {
      return;
    }
    event.preventDefault();
    onSelect(item.id, isMultiSelect(event));
  };

  return (
    <article
      className={`flex flex-col rounded-lg border bg-card text-card-foreground shadow-sm transition-colors ${
        selected ? 'border-primary ring-1 ring-primary' : 'border-border'
      }`}
      data-selected={selected ? 'true' : 'false'}
      onClick={select}
    >
      <div className="flex gap-3 p-3">
        <ItemThumbnail item={item} />

        <div className="min-w-0 flex-1">
          <div className="flex items-start justify-between gap-2">
            <div className="min-w-0">
              {/*
                The title is a link to the detail page of the item. The
                link is the whole title, so that the click lands on the
                words the reader aimed at.
              */}
              <h3 className="truncate text-sm font-semibold leading-tight">
                <Link
                  to={`/wikidata/${item.id}`}
                  className="hover:underline"
                  title={`Open ${item.label.text || item.id}`}
                >
                  {item.label.text || item.id}
                </Link>
              </h3>
              <p className="mt-0.5 flex items-center gap-1.5 text-xs text-muted-foreground">
                <a
                  className="font-mono hover:text-foreground hover:underline"
                  href={entityUrl(item.id)}
                  target="_blank"
                  rel="noreferrer noopener"
                >
                  {item.id}
                </a>
                {/* The label of an item comes from one language. When it
                    is the fallback, the card says so, because an
                    English reader needs to know that the label is
                    French. */}
                {item.label.languageKind === "fallback" ? (
                  <span className="rounded bg-muted px-1 py-0.5 text-[0.65rem] uppercase">
                    {item.label.language}
                  </span>
                ) : null}
              </p>
            </div>

            <button
              type="button"
              className="shrink-0 rounded-md p-1 text-muted-foreground hover:bg-accent hover:text-foreground"
              onClick={toggle}
              aria-expanded={expanded}
              aria-label={expanded ? "Hide the details" : "Show the details"}
            >
              {expanded ? (
                <ChevronDown className="h-4 w-4" aria-hidden="true" />
              ) : (
                <ChevronRight className="h-4 w-4" aria-hidden="true" />
              )}
            </button>
          </div>

          {item.description.text ? (
            <p className="mt-1 line-clamp-2 text-xs text-muted-foreground">{item.description.text}</p>
          ) : null}

          <CardFooter item={item} places={places} />
        </div>
      </div>

      {expanded ? (
        <div className="border-t border-border">
          <PropertyTable item={item} />
          {showGraph ? (
            <div className="border-t border-border p-3">
              <EntityGraph item={item} />
            </div>
          ) : null}
        </div>
      ) : null}
    </article>
  );
}

/**
 * The thumbnail of an item.
 *
 * The dump holds the name of the file of P18, not the file itself, so
 * the image comes from Wikimedia Commons. Without a network, the
 * placeholder is shown instead.
 */
function ItemThumbnail({ item }: { readonly item: EntityView }) {
  const image = item.imageId;

  return (
    <div className="flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-md bg-muted">
      {image ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img
          className="h-full w-full object-cover"
          src={`https://commons.wikimedia.org/wiki/Special:Redirect/file/${encodeURIComponent(image)}?width=200`}
          alt={item.label.text || item.id}
          loading="lazy"
        />
      ) : (
        <span className="text-xs text-muted-foreground">{item.id}</span>
      )}
    </div>
  );
}

/**
 * The summary line of a card: the places and the count of the claims.
 */
function CardFooter({
  item,
  places,
}: {
  readonly item: EntityView;
  readonly places: readonly { placeQid: string; propertyKind: "filming" | "narrative" }[];
}) {
  return (
    <div className="mt-2 flex flex-wrap items-center gap-1.5">
      {places.slice(0, 4).map((place) => (
        <span
          key={`${place.propertyKind}-${place.placeQid}`}
          className="inline-flex items-center gap-1 rounded border border-border px-1.5 py-0.5 text-[0.65rem] text-muted-foreground"
          title={
            place.propertyKind === "filming"
              ? "A place where the work was filmed"
              : "A place where the story takes place"
          }
        >
          <span
            className={
              place.propertyKind === "filming"
                ? "rounded bg-emerald-500/15 px-1 text-emerald-600 dark:text-emerald-400"
                : "rounded bg-sky-500/15 px-1 text-sky-600 dark:text-sky-400"
            }
          >
            {place.propertyKind === "filming" ? "film" : "story"}
          </span>
          {place.placeQid}
        </span>
      ))}
      {places.length > 4 ? (
        <span className="text-[0.65rem] text-muted-foreground">+{places.length - 4}</span>
      ) : null}

      <span className="ml-auto text-[0.65rem] text-muted-foreground">
        {item.claimCount} {item.claimCount === 1 ? "property" : "properties"}
      </span>
    </div>
  );
}

/**
 * The table of the properties of an item.
 *
 * One row per property, and under it the statements of that property.
 * The qualifiers and the references of a statement are shown when the
 * row is opened, so that the table stays readable.
 */
export function PropertyTable({ item }: { readonly item: EntityView }) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full border-collapse text-sm">
        <tbody>
          {item.claims.map((claim) => (
            <ClaimRow key={claim.propertyId} claim={claim} />
          ))}
        </tbody>
      </table>
    </div>
  );
}

/**
 * One row of the property table.
 */
function ClaimRow({ claim }: { readonly claim: EntityView["claims"][number] }) {
  return (
    <>
      {claim.statements.map((statement, index) => (
        <StatementRow
          key={statement.id}
          statement={statement}
          propertyId={claim.propertyId}
          propertyLabel={claim.propertyLabel}
          showProperty={index === 0}
        />
      ))}
    </>
  );
}

/**
 * One statement of the property table.
 */
function StatementRow({
  statement,
  propertyId,
  propertyLabel,
  showProperty,
}: {
  readonly statement: EntityView["claims"][number]["statements"][number];
  readonly propertyId: string;
  readonly propertyLabel: string;
  readonly showProperty: boolean;
}) {
  const [open, setOpen] = useState(false);

  const hasDetails = statement.qualifiers.length > 0 || statement.references.length > 0;

  return (
    <>
      <tr className="border-b border-border/60 align-top last:border-b-0">
        <th
          scope="row"
          className="w-44 whitespace-nowrap py-1.5 pl-3 pr-2 text-left text-xs font-normal text-muted-foreground"
        >
          {showProperty ? (
            <>
              <span className="font-medium text-foreground">{propertyLabel}</span>
              <span className="ml-1 font-mono text-[0.65rem] opacity-60">{propertyId}</span>
            </>
          ) : null}
        </th>

        <td className="py-1.5 pr-3">
          <div className="flex flex-wrap items-center gap-1.5">
            {statement.value ? <ValueRenderer value={statement.value} /> : null}

            {statement.rank !== "normal" ? (
              <span
                className={
                  statement.rank === "preferred"
                    ? "rounded bg-emerald-500/15 px-1.5 py-0.5 text-[0.65rem] text-emerald-600 dark:text-emerald-400"
                    : "rounded bg-muted px-1.5 py-0.5 text-[0.65rem] text-muted-foreground line-through"
                }
              >
                {statement.rank}
              </span>
            ) : null}

            {hasDetails ? (
              <button
                type="button"
                className="rounded px-1 py-0.5 text-[0.65rem] text-muted-foreground hover:bg-accent hover:text-foreground"
                onClick={() => setOpen((value) => !value)}
                aria-expanded={open}
              >
                {open ? "less" : "details"}
              </button>
            ) : null}
          </div>
        </td>
      </tr>

      {open ? (
        <tr className="border-b border-border/60 bg-muted/30">
          <td colSpan={2} className="px-3 py-2">
            <StatementDetails statement={statement} />
          </td>
        </tr>
      ) : null}
    </>
  );
}

/**
 * The qualifiers and the references of a statement.
 */
function StatementDetails({
  statement,
}: {
  readonly statement: EntityView["claims"][number]["statements"][number];
}) {
  return (
    <div className="space-y-2 text-xs">
      {statement.qualifiers.length > 0 ? (
        <div>
          <p className="mb-1 font-medium text-muted-foreground">Qualifiers</p>
          <ul className="space-y-1">
            {statement.qualifiers.map((qualifier, index) => (
              <li key={`${qualifier.propertyId}-${index}`} className="flex gap-2">
                <span className="w-40 shrink-0 text-muted-foreground">{qualifier.propertyLabel}</span>
                <ValueRenderer value={qualifier.value} />
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {statement.references.length > 0 ? (
        <div>
          <p className="mb-1 font-medium text-muted-foreground">References</p>
          <ul className="space-y-1">
            {statement.references.map((reference, index) => (
              <li key={index} className="flex flex-wrap gap-x-2 gap-y-1">
                {reference.entries.map((entry, entryIndex) => (
                  <span key={`${entry.propertyId}-${entryIndex}`} className="inline-flex items-center gap-1">
                    <span className="text-muted-foreground">{entry.propertyLabel}:</span>
                    <ValueRenderer value={entry.value} />
                  </span>
                ))}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

/**
 * The link to the entity page of Wikidata.
 */
export function WikidataLink({ id }: { readonly id: string }) {
  return (
    <a
      className="inline-flex items-center gap-1 font-mono text-xs text-primary hover:underline"
      href={entityUrl(id)}
      target="_blank"
      rel="noreferrer noopener"
    >
      {id}
      <ExternalLink className="h-3 w-3" aria-hidden="true" />
    </a>
  );
}
