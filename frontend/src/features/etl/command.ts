/**
 * The pure helpers of the ETL panel.
 *
 * They are kept out of the component, and they are tested on their
 * own: the parsing of the parameters and the formatting of the
 * counters are the parts that can be wrong without the interface
 * showing it.
 */

/**
 * The status of a run.
 */
export type EtlStatus = 'running' | 'succeeded' | 'cancelled' | 'failed';

/**
 * Parses the languages of the parameter field.
 *
 * The field holds the codes separated by commas: "en,zh,fr". The
 * codes are trimmed and the empty ones are dropped, so that a trailing
 * comma does not produce an empty language.
 */
export function parseLanguages(value: string): string[] {
  return value
    .split(',')
    .map((language) => language.trim())
    .filter((language) => language.length > 0);
}

/**
 * Writes the languages back into the parameter field.
 *
 * The round trip has to hold: the panel shows the languages of the
 * last run, and a trailing comma would show up in the command.
 */
export function formatLanguages(languages: string[]): string {
  return languages.join(',');
}

/**
 * Tells whether a language code looks like one.
 *
 * The check is the shape of the code: a lower case letter, or a
 * letter and a hyphenated part. It is not the list of the real
 * languages: the dump holds codes that the standards do not list.
 */
export function isLanguageCode(value: string): boolean {
  if (!value || value.length > 12) {
    return false;
  }

  const parts = value.split('-');
  if (parts.length > 3) {
    return false;
  }

  return parts.every((part) => part.length > 0 && /^[a-z0-9]+$/.test(part));
}

/**
 * Tells whether a target looks like a Wikidata identifier.
 *
 * An identifier is a Q followed by digits: Q90 is Paris. A target
 * that is not one matches nothing, and a run that matches nothing
 * looks like a broken dump.
 */
export function isQID(value: string): boolean {
  return /^Q[0-9]+$/.test(value.trim());
}

/**
 * Formats the number of lines for the display.
 *
 * The counters of a run over a hundred gigabytes are large, so they
 * are shown with a thousands separator.
 */
export function formatCount(value: number): string {
  return new Intl.NumberFormat('en-US').format(value);
}

/**
 * Formats the speed for the display.
 *
 * A speed under one megabyte per second is shown with a decimal: the
 * run over a small dump is slow, and "0 MB/s" would hide that.
 */
export function formatSpeed(megabytesPerSecond: number): string {
  if (megabytesPerSecond <= 0) {
    return '—';
  }
  if (megabytesPerSecond < 1) {
    return `${megabytesPerSecond.toFixed(2)} MB/s`;
  }
  return `${megabytesPerSecond.toFixed(1)} MB/s`;
}

/**
 * Describes what a status means, for the interface.
 */
export function statusLabel(status: EtlStatus | string): string {
  switch (status) {
    case 'running':
      return 'Running';
    case 'succeeded':
      return 'Succeeded';
    case 'cancelled':
      return 'Cancelled';
    case 'failed':
      return 'Failed';
    default:
      return 'Unknown';
  }
}

/**
 * Tells whether a run is still going.
 */
export function isRunning(status: EtlStatus | string): boolean {
  return status === 'running';
}
