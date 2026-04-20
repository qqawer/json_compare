import { defineConfig } from 'vite'

// Vite dev proxy: forward /compare to backend at localhost:8080
export default defineConfig({
  server: {
    proxy: {
      '/compare': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
    },
  },
})
