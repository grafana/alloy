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
 * Available mock fixtures mapped by their localID.
 */
const MOCK_FIXTURES = [
  'discovery.kubernetes.heavy',
  'discovery.kubernetes.light',
];

/**
 * Helper to load a fixture by component ID.
 */
function loadFixture(componentId: string): string | null {
  try {
    return readFileSync(
      join(__dirname, `src/test/generated_fixtures/${componentId}.json`),
      'utf-8'
    );
  } catch {
    return null;
  }
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
          const components = [];
          for (const fixtureId of MOCK_FIXTURES) {
            const fixture = loadFixture(fixtureId);
            if (fixture) {
              const data = JSON.parse(fixture);
              components.push({
                moduleID: data.moduleID || '',
                localID: data.localID,
                name: data.name,
                label: data.label,
                health: data.health,
                referencedBy: data.referencedBy || [],
                referencesTo: data.referencesTo || [],
                dataFlowEdgesTo: data.dataFlowEdgesTo || [],
                liveDebuggingEnabled: false,
              });
            }
          }
          res.end(JSON.stringify(components));
          return;
        }

        // Mock: GET /api/v0/web/components/:id (component detail)
        if (req.url.startsWith('/api/v0/web/components/')) {
          const componentId = req.url.replace('/api/v0/web/components/', '');
          res.setHeader('Content-Type', 'application/json');
          const fixture = loadFixture(componentId);
          if (fixture) {
            res.end(fixture);
          } else {
            res.statusCode = 404;
            res.end(JSON.stringify({ error: `Fixture not found: ${componentId}` }));
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
