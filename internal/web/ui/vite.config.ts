import react from '@vitejs/plugin-react';
import { readFileSync } from 'fs';
import { join } from 'path';
import { defineConfig, type PluginOption } from 'vite';

/**
 * Plugin to inject Go template syntax for runtime base path. Only runs during
 * build, not during dev.
 * @returns Vite plugin.
 */
function goTemplatePlugin() {
  return {
    name: 'go-template',
    apply: 'build', // Only apply this plugin during build, not dev
    transformIndexHtml(html: string) {
      // Replace the base tag with Go template syntax for runtime injection
      return html.replace(/<base href="\/" \/>/, '<base href="{{! .PublicURL !}}/" />');
    },
  } satisfies PluginOption;
}

/**
 * Plugin to mock the Alloy API during development.
 * Serves fixture data from src/test/fixtures/ for testing without a real Alloy instance.
 *
 * To use: place JSON files in src/test/fixtures/ and run `npm run dev:mock`
 */
function mockApiPlugin(): PluginOption {
  return {
    name: 'mock-api',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        // Only intercept API requests
        if (!req.url?.startsWith('/api/')) {
          return next();
        }

        // Mock: GET /api/v0/web/components (list all components)
        if (req.url === '/api/v0/web/components') {
          res.setHeader('Content-Type', 'application/json');
          try {
            const fixture = readFileSync(
              join(__dirname, 'src/test/fixtures/large_disc_output.json'),
              'utf-8'
            );
            const data = JSON.parse(fixture);
            // Return as a list with one component
            res.end(JSON.stringify([{
              moduleID: data.moduleID || '',
              localID: data.localID,
              name: data.name,
              label: data.label,
              health: data.health,
              referencedBy: data.referencedBy || [],
              referencesTo: data.referencesTo || [],
              dataFlowEdgesTo: data.dataFlowEdgesTo || [],
              liveDebuggingEnabled: false,
            }]));
          } catch {
            res.end(JSON.stringify([]));
          }
          return;
        }

        // Mock: GET /api/v0/web/components/:id (component detail)
        if (req.url.startsWith('/api/v0/web/components/')) {
          res.setHeader('Content-Type', 'application/json');
          try {
            const fixture = readFileSync(
              join(__dirname, 'src/test/fixtures/large_disc_output.json'),
              'utf-8'
            );
            res.end(fixture);
          } catch (err) {
            res.statusCode = 404;
            res.end(JSON.stringify({ error: 'Fixture not found' }));
          }
          return;
        }

        // Pass through other API requests
        next();
      });
    },
  };
}

// https://vite.dev/config/
export default defineConfig(({ mode }) => ({
  plugins: [
    react(),
    goTemplatePlugin(),
    // Only enable mock API in development when MOCK env var is set
    mode === 'development' && process.env.MOCK === 'true' && mockApiPlugin(),
  ].filter(Boolean),
  // Use relative base path so assets work when served from any path prefix
  // The <base> tag in index.html will be set at runtime by the Go server
  base: './',
}));
