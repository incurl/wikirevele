/**
 * The renderers of the Wikidata datatypes.
 *
 * Every renderer takes a resolved value and shows it. The resolved
 * value says which kind it is, so the renderer below picks the right
 * one. A value whose kind has no renderer is shown as its raw text,
 * which is better than hiding it.
 */

import { ExternalLink, Hash, Image as ImageIcon } from "lucide-react";

import type { ResolvedValue, TimeParts } from "../view";
import { entityUrl } from "../view";

/**
 * The props of every renderer.
 */
export interface ValueRendererProps {
  /** The value to show. */
  readonly value: ResolvedValue;
}

/**
 * The QID chip: a link to the entity page of the identifier.
 *
 * The label of the entity is not in the subset, so the chip shows the
 * identifier itself.
 */
export function EntityValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "entity") {
    return null;
  }

  return (
    <a
      className="inline-flex items-center gap-1 rounded-md border border-border bg-muted px-1.5 py-0.5 font-mono text-xs text-foreground no-underline hover:bg-accent"
      href={entityUrl(value.id)}
      target="_blank"
      rel="noreferrer noopener"
    >
      <Hash className="h-3 w-3" aria-hidden="true" />
      {value.id}
    </a>
  );
}

/**
 * A plain text value.
 */
export function TextValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "text") {
    return null;
  }
  return <span className="text-sm">{value.text}</span>;
}

/**
 * A URL value.
 */
export function UrlValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "url") {
    return null;
  }
  return (
    <a
      className="inline-flex items-center gap-1 text-sm text-primary underline underline-offset-4 hover:opacity-80"
      href={value.url}
      target="_blank"
      rel="noreferrer noopener"
    >
      <ExternalLink className="h-3 w-3 shrink-0" aria-hidden="true" />
      <span className="break-all">{value.url}</span>
    </a>
  );
}

/**
 * An external identifier, with a link to the resolver when the panel
 * knows one.
 */
export function ExternalIdValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "external-id") {
    return null;
  }

  if (!value.url) {
    return <span className="font-mono text-sm">{value.id}</span>;
  }

  return (
    <a
      className="font-mono text-sm text-primary underline underline-offset-4 hover:opacity-80"
      href={value.url}
      target="_blank"
      rel="noreferrer noopener"
    >
      {value.id}
    </a>
  );
}

/**
 * A text in one language, with the language as a badge.
 */
export function MonolingualValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "monolingual") {
    return null;
  }

  return (
    <span className="inline-flex items-center gap-1.5">
      <span className="text-sm">{value.text}</span>
      <span className="rounded bg-muted px-1 py-0.5 text-[0.65rem] uppercase text-muted-foreground">
        {value.language}
      </span>
    </span>
  );
}

/**
 * The number of the unit of a quantity, with the amount.
 *
 * The unit is a URL in the dump. The panel shows the last part of it,
 * which is the readable name of the unit.
 */
export function QuantityValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "quantity") {
    return null;
  }

  const unit = unitName(value.unit);
  return (
    <span className="inline-flex items-baseline gap-1 text-sm">
      <span className="font-medium tabular-nums">{value.amount}</span>
      {unit ? <span className="text-muted-foreground">{unit}</span> : null}
    </span>
  );
}

/**
 * The readable name of a unit, from its URL.
 */
export function unitName(unit: string): string {
  const last = unit.split("/").pop() ?? "";
  if (!last || last === "1") {
    return "";
  }
  return last.replace(/-/g, " ");
}

/**
 * The months, in the order the calendar uses.
 */
const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

/**
 * Formats a time value at the precision it was recorded with.
 *
 * A year is shown as a year, and a day is shown as a day. Showing more
 * than the precision allows would be a lie about the data.
 */
export function formatTime(time: TimeParts): string {
  const year = time.year < 0 ? `${-time.year} BCE` : `${time.year}`;
  const month = time.month >= 1 && time.month <= 12 ? MONTHS[time.month - 1] : "";

  let text: string;
  if (time.precision === 0) {
    text = year;
  } else if (time.precision === 1) {
    text = month ? `${month} ${year}` : year;
  } else if (time.precision === 2) {
    text = time.day ? `${time.day} ${month} ${year}` : year;
  } else {
    text = time.day ? `${time.day} ${month} ${year}` : year;
  }

  if (time.before) {
    text = `before ${text}`;
  }
  if (time.after) {
    text = `after ${text}`;
  }
  return text;
}

/**
 * A time value, shown at its precision.
 */
export function TimeValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "time") {
    return null;
  }

  return (
    <span className="inline-flex items-center gap-1.5 text-sm">
      <span>{formatTime(value.time)}</span>
      {value.time.precision < 11 ? (
        <span
          className="rounded bg-muted px-1 py-0.5 text-[0.65rem] text-muted-foreground"
          title="The precision the value was recorded with"
        >
          {`±${value.time.precision}`}
        </span>
      ) : null}
    </span>
  );
}

/**
 * A coordinate, with a small map.
 *
 * The map is drawn here, in the page: the panel runs inside a webview
 * with no network access, so it cannot fetch map tiles. The preview
 * plots the point on a plain grid, which is enough to see where a
 * place is.
 */
export function CoordinateValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "coordinate") {
    return null;
  }

  return (
    <span className="inline-flex items-center gap-2">
      <CoordinatePreview latitude={value.latitude} longitude={value.longitude} />
      <span className="font-mono text-xs tabular-nums text-muted-foreground">
        {value.latitude.toFixed(4)}, {value.longitude.toFixed(4)}
      </span>
    </span>
  );
}

/**
 * A small map of a point.
 *
 * The projection is a plain equirectangular one: the longitude is the
 * horizontal axis and the latitude the vertical one. It is not a
 * projection to navigate with, and it does not pretend to be.
 */
export function CoordinatePreview({
  latitude,
  longitude,
}: {
  readonly latitude: number;
  readonly longitude: number;
}) {
  const width = 72;
  const height = 48;

  // The equirectangular projection maps the longitude to the width and
  // the latitude to the height.
  const x = ((longitude + 180) / 360) * width;
  const y = ((90 - latitude) / 180) * height;

  return (
    <svg
      className="rounded border border-border bg-muted"
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      role="img"
      aria-label={`Map of ${latitude.toFixed(4)}, ${longitude.toFixed(4)}`}
    >
      {/* The equator and the prime meridian, so that the position of
          the point can be read. */}
      <line x1={0} y1={height / 2} x2={width} y2={height / 2} stroke="currentColor" strokeOpacity={0.15} />
      <line x1={width / 2} y1={0} x2={width / 2} y2={height} stroke="currentColor" strokeOpacity={0.15} />
      <circle cx={x} cy={y} r={3} fill="hsl(var(--primary))" />
    </svg>
  );
}

/**
 * A statement that says the link does not exist, or that it exists but
 * is not known.
 */
export function EmptyValueRenderer({ value }: ValueRendererProps) {
  if (value.kind !== "no-value" && value.kind !== "some-value") {
    return null;
  }

  const text = value.kind === "no-value" ? "no value" : "some value";
  return (
    <span className="rounded bg-muted px-1.5 py-0.5 text-xs italic text-muted-foreground">{text}</span>
  );
}

/**
 * The image of an item, shown as a thumbnail.
 *
 * The dump holds the name of the file, not the file itself, so the
 * thumbnail is loaded from Wikimedia Commons. When there is no network
 * the image simply does not appear.
 */
export function ItemImage({
  fileName,
  alt,
  className,
}: {
  readonly fileName: string;
  readonly alt: string;
  readonly className?: string;
}) {
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      className={className}
      src={`https://commons.wikimedia.org/wiki/Special:Redirect/file/${encodeURIComponent(fileName)}?width=200`}
      alt={alt}
      loading="lazy"
    />
  );
}

/**
 * The placeholder of an item that has no image.
 */
export function ItemImagePlaceholder({ className }: { readonly className?: string }) {
  return (
    <div
      className={`flex items-center justify-center bg-muted text-muted-foreground ${className ?? ""}`}
      aria-hidden="true"
    >
      <ImageIcon className="h-5 w-5" />
    </div>
  );
}

/**
 * Renders a value of any kind.
 */
export function ValueRenderer({ value }: ValueRendererProps) {
  switch (value.kind) {
    case "entity":
      return <EntityValueRenderer value={value} />;
    case "text":
      return <TextValueRenderer value={value} />;
    case "url":
      return <UrlValueRenderer value={value} />;
    case "external-id":
      return <ExternalIdValueRenderer value={value} />;
    case "monolingual":
      return <MonolingualValueRenderer value={value} />;
    case "time":
      return <TimeValueRenderer value={value} />;
    case "quantity":
      return <QuantityValueRenderer value={value} />;
    case "coordinate":
      return <CoordinateValueRenderer value={value} />;
    case "no-value":
    case "some-value":
      return <EmptyValueRenderer value={value} />;
  }
}
