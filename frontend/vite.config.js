import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src')
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: process.env.BLOG_PROXY_TARGET || 'http://localhost:8080',
        changeOrigin: true
      },
      '/agent-api': {
        target: process.env.AGENT_PROXY_TARGET || 'http://localhost:8000',
        changeOrigin: true,
        rewrite: path => path.replace(/^\/agent-api/, '')
      }
    }
  }
})
