import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const proxyTarget = process.env.VITE_PROXY_TARGET || 'http://127.0.0.1:8080'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': {
        target: proxyTarget,
        changeOrigin: true,
        ws: false
      },
      '/healthz': proxyTarget
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 1600,
    rollupOptions: {
      output: {
        manualChunks: {
          vue: ['vue', 'vue-router', 'pinia', 'vue-i18n'],
          element: ['element-plus', '@element-plus/icons-vue'],
          charts: ['echarts']
        }
      }
    }
  }
})
