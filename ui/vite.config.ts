import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // The Go server owns the API under /api; `npm run dev` proxies it so the
    // UI is same-origin in development, exactly as the nginx LB makes it in
    // docker compose and the ingress makes it in production.
    proxy: {
      '/api': 'http://localhost:4000',
    },
  },
})
