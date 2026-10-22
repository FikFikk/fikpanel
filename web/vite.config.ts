import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';
import viteCompression from 'vite-plugin-compression';

export default defineConfig({
  plugins: [
    svelte(),
    // Kompresi aset gzip untuk disajikan langsung oleh Go
    viteCompression({ algorithm: 'gzip', ext: '.gz' }),
    // Kompresi aset brotli untuk performa transfer maksimal
    viteCompression({ algorithm: 'brotliCompress', ext: '.br' })
  ],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    target: 'esnext'
  }
});
