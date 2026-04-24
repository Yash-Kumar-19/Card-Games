import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
        configure(proxy) {
          // Suppress EPIPE/ECONNRESET noise caused by the browser closing
          // the WS before Vite finishes forwarding (harmless during dev).
          proxy.on('error', () => {})
          proxy.on('proxyReqWs', (_req, _socket, _head, target) => {
            (target as import('net').Socket).on?.('error', () => {})
          })
        },
      },
    },
  },
})
