import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import {resolve} from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  build: {
    rollupOptions: {
      // Two pages sharing one build: index.html is the main card-rail panel, settings.html is
      // the standalone settings window (see main.go's runSettingsWindow, which serves
      // settings.html in place of index.html for that second process). Both land in the same
      // frontend/dist, with distinct content-hashed asset filenames, so a single go:embed covers
      // both.
      input: {
        main: resolve(__dirname, 'index.html'),
        settings: resolve(__dirname, 'settings.html'),
      },
    },
  },
})
