import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import wails from '@wailsio/runtime/plugins/vite';
import { fileURLToPath, URL } from 'node:url';

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: '127.0.0.1',
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react(), wails('./bindings')],
  resolve: {
    alias: {
      // Add the module specifiers of your own vendored packages here.
      '@app': fileURLToPath(new URL('./src/app', import.meta.url)),
      // The shadcn/ui components and the Refine components import
      // through the "@/" prefix, as the shadcn CLI writes them.
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
});
