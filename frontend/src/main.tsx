import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import ReactDOM from 'react-dom/client';

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

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ErrorBoundary>
        <App />
      </ErrorBoundary>
    </QueryClientProvider>
  </React.StrictMode>,
);
