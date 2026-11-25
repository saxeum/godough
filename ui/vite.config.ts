import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // The Go server owns the API; `npm run dev` proxies it so the UI can call
    // /snippet/* in development without CORS.
    proxy: {
      '/snippet': 'http://localhost:4000',
    },
  },
})
