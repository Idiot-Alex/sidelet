import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  plugins: [svelte()],
  base: './',
  resolve: { dedupe: ['svelte', '@wailsio/runtime'] },
  server: { port: 5174, strictPort: true, fs: { allow: [fileURLToPath(new URL('..', import.meta.url))] } },
  preview: { port: 5174, strictPort: true },
  build: { target: 'es2022' },
});
