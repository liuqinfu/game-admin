import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) {
            return
          }

          if (id.includes('react-dom') || id.includes('react-router') || id.includes('/react/')) {
            return 'react-vendor'
          }

          if (id.includes('@tanstack/react-query')) {
            return 'query-vendor'
          }

          if (id.includes('antd/es/table') || id.includes('rc-table') || id.includes('rc-pagination') || id.includes('rc-resize-observer')) {
            return 'antd-table'
          }

          if (id.includes('@ant-design/icons')) {
            return 'antd-icons'
          }

          if (
            id.includes('/antd/') ||
            id.includes('/rc-') ||
            id.includes('@ant-design')
          ) {
            return 'antd-core'
          }
        },
      },
    },
  },
})
