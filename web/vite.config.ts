import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import monacoEditorPlugin from 'vite-plugin-monaco-editor'
import { resolve } from 'path'

export default defineConfig({
  plugins: [
    vue(),
    (monacoEditorPlugin as any).default({
      languageWorkers: ['editorWorkerService', 'json'],
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  build: {
    chunkSizeWarningLimit: 1000,
    rollupOptions: {
      output: {
        manualChunks: {
          'monaco-editor': ['monaco-editor'],
        },
      },
    },
  },
  server: {
    port: Number(process.env.VITE_PORT) || 5173,
    proxy: {
      '/api/v1/ws': {
        target: process.env.VITE_API_TARGET?.replace('http', 'ws') || 'ws://localhost:3678',
        ws: true,
      },
      '/api': {
        target: process.env.VITE_API_TARGET || 'http://localhost:3678',
        changeOrigin: true,
      },
    },
  },
})
