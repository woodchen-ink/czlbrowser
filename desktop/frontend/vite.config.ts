import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// wails dev 连到这个地址 (wails.json frontend:dev:serverUrl), 端口改了两边一起改
const DEV_HOST = '127.0.0.1'
const DEV_PORT = 5218

export default defineConfig({
  plugins: [react()],
  server: {
    host: DEV_HOST,
    port: DEV_PORT,
    strictPort: true,
    hmr: { host: DEV_HOST, protocol: 'ws' },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        // React 全家桶单独成块, 业务代码更新时这块缓存不变
        manualChunks: { react: ['react', 'react-dom', 'react-router-dom'] },
      },
    },
  },
})
