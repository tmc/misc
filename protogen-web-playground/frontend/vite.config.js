import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import topLevelAwait from 'vite-plugin-top-level-await';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [
    react(),
    topLevelAwait({
      // The export name of top-level await promise for each chunk module
      promiseExportName: '__tla',
      // The function to generate import names of top-level await promise in each chunk module
      promiseImportName: i => `__tla_${i}`
    })
  ],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        secure: false,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      }
    }
  },
  optimizeDeps: {
    esbuildOptions: {
      // Enable esbuild polyfill for WebAssembly
      plugins: [
        {
          name: 'wasm-loader',
          setup(build) {
            // Handle .wasm imports
            build.onResolve({ filter: /\.wasm$/ }, args => {
              return { path: args.path, namespace: 'wasm-loader' }
            });
            build.onLoad({ filter: /.*/, namespace: 'wasm-loader' }, async (args) => {
              return {
                contents: `
                  export default (opts = {}) => WebAssembly.instantiate(
                    fetch('${args.path}').then(r => r.arrayBuffer()), opts
                  )
                `,
                loader: 'js'
              }
            });
          }
        }
      ]
    }
  },
  build: {
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'monaco-editor': ['monaco-editor'],
        }
      }
    }
  }
});