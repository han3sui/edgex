import { defineConfig } from 'vitest/config'
import path from 'path'

export default defineConfig({
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      api: path.resolve(__dirname, './src/api'),
      stores: path.resolve(__dirname, './src/stores')
    }
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.spec.js']
  }
})
