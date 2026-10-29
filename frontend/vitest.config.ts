import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    // The tests that touch the DOM need an environment to assert
    // against.
    environment: 'jsdom',
    include: ['src/**/*.test.ts', 'src/**/*.test.tsx'],
  },
});
