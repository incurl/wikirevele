import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import ReactDOM from 'react-dom/client';
import { createBrowserRouter, RouterProvider } from 'react-router';

import App from './app/App';
import { ErrorBoundary } from './app/ErrorBoundary';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // The Wails event stream pushes job updates; the refetch interval
      // is only a safety net, so retries stay off.
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

/**
 * The routes of the application.
 *
 * The shell is the root: it holds the rails and the canvas. The
 * Wikidata panel has routes of its own, and the canvas renders it
 * where it belongs.
 */
const router = createBrowserRouter([
  {
    path: '/',
    element: <App />,
    children: [
      {
        path: 'wikidata',
        element: <App />,
      },
      {
        path: 'wikidata/:id',
        element: <App />,
      },
    ],
  },
]);

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ErrorBoundary>
        <RouterProvider router={router} />
      </ErrorBoundary>
    </QueryClientProvider>
  </React.StrictMode>,
);
