import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// /api 代理到 fun 后端（POST /api/cell → 后端 /cell）
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9200',
        changeOrigin: true,
        rewrite: p => p.replace(/^\/api/, ''),
      },
    },
  },
})
