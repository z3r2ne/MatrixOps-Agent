import path from 'path';
import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => {
    const env = loadEnv(mode, '.', '');
    return {
      server: {
        port: 3010,
        host: 'localhost',
        proxy: {
          '/api': {
            target: 'http://localhost:8080',
            changeOrigin: true,
            ws: true, // Enable WebSocket proxy
          },
        },
      },
  plugins: [react()],
  optimizeDeps: {
    include: ['@base-ui/react'],
  },
  build: {
    outDir: '../web-server/web/dist',
    emptyOutDir: true,
  },
      define: {
        'process.env.API_KEY': JSON.stringify(env.GEMINI_API_KEY),
        'process.env.GEMINI_API_KEY': JSON.stringify(env.GEMINI_API_KEY)
      },
      resolve: {
        alias: {
          '@': path.resolve(__dirname, './src'),
        }
      }
    };
});
