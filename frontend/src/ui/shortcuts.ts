/**
 * The keyboard shortcuts of the application.
 *
 * The webview has no menu bar of its own, so the shortcuts are matched
 * in the page. Only the ones the application defines are handled:
 * everything else is left to the webview, so that the shortcuts of the
 * platform keep working.
 */

/**
 * Reports whether a key event asks for the debug console.
 *
 * Ctrl+Shift+I is the shortcut the browsers use for their developer
 * tools, and it fits the debug console. The Command key is accepted as
 * well, because that is the modifier a macOS user reaches for.
 */
export function isConsoleShortcut(event: KeyboardEvent): boolean {
  if (!event.shiftKey) {
    return false;
  }

  if (!event.ctrlKey && !event.metaKey) {
    return false;
  }

  return event.key.toLowerCase() === 'i';
}
