import path from "path"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { VitePWA } from 'vite-plugin-pwa'

// https://vite.dev/config/
export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    VitePWA({
      // Custom worker (src/pwa-sw.ts): generateSW's runtimeCaching can't
      // share one BackgroundSync queue across POST/PUT/DELETE routes.
      strategies: 'injectManifest',
      srcDir: 'src',
      filename: 'pwa-sw.ts',
      registerType: 'autoUpdate',
      // The plugin generates manifest.webmanifest and injects its <link> into
      // index.html (this replaced the old hand-written public/manifest.json).
      manifest: {
        name: 'RepLog',
        short_name: 'RepLog',
        description: 'Self-hosted workout tracking for the family',
        start_url: '/',
        display: 'standalone',
        background_color: '#1a1a1a',
        theme_color: '#1a1a1a',
        categories: ['fitness', 'sports'],
        icons: [
          { src: '/icon.svg', sizes: 'any', type: 'image/svg+xml' },
          { src: '/pwa-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: '/pwa-512x512.png', sizes: '512x512', type: 'image/png' },
          { src: '/pwa-maskable-512x512.png', sizes: '512x512', type: 'image/png', purpose: 'maskable' },
        ],
      },
      injectManifest: {
        // App shell precache: hashed js/css plus icons and fonts.
        globPatterns: ['**/*.{js,css,html,ico,png,svg,woff2}'],
        maximumFileSizeToCacheInBytes: 3 * 1024 * 1024,
      },
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    // Proxy backend requests to the Go binary during development.
    // Auth, API, and avatar requests all go to :8080.
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        timeout: 300000, // 5 minutes for LLM generation
      },
      // OIDC relying-party endpoints (ADR 019 Phase 1) are full-page
      // browser redirects served by the Go binary, not XHR under /api.
      // Scope narrowly to /auth/oidc — /auth/token/:token is a client-side
      // SPA route, not a backend endpoint.
      '/auth/oidc': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/avatars': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        // vite 8 (rolldown) dropped the object form of manualChunks; use the
        // function form to keep the same three vendor chunks.
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined
          if (/[\\/]node_modules[\\/](react|react-dom|react-router|react-router-dom|scheduler)[\\/]/.test(id)) {
            return 'vendor-react'
          }
          if (/[\\/]node_modules[\\/]@tanstack[\\/]/.test(id)) {
            return 'vendor-query'
          }
          if (/[\\/]node_modules[\\/](@base-ui[\\/]react|class-variance-authority|lucide-react)[\\/]/.test(id)) {
            return 'vendor-ui'
          }
          return undefined
        },
      },
    },
  },
})
