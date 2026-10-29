/**
 * The application error boundary.
 *
 * A render error in a browser leaves the user with a reload button. A
 * desktop application has no browser chrome, so an uncaught error would
 * leave a blank window with no way back. The boundary catches it, shows
 * what happened, and offers a reload.
 */

import { Component } from 'react';
import type { ErrorInfo, ReactNode } from 'react';

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
}

/**
 * Renders a fallback instead of a blank window when a child throws.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    // The webview has a console, and it is the only place a developer
    // can see the component stack.
    console.error('The application failed to render:', error, info.componentStack);
  }

  render(): ReactNode {
    const { error } = this.state;
    if (error === null) {
      return this.props.children;
    }

    return (
      <div className="crash">
        <div className="crash-panel">
          <h1>The interface stopped working</h1>
          <p className="text-dim">
            An unexpected error stopped the window from rendering. Nothing has
            been lost: the jobs are stored in the local database.
          </p>
          <pre className="crash-message">{error.message}</pre>
          <div className="crash-actions">
            <button
              type="button"
              className="btn btn-primary"
              onClick={() => window.location.reload()}
            >
              Reload the window
            </button>
            <button
              type="button"
              className="btn"
              onClick={() => this.setState({ error: null })}
            >
              Try again
            </button>
          </div>
        </div>
      </div>
    );
  }
}
