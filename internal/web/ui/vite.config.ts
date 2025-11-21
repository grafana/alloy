import react from '@vitejs/plugin-react';
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

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), goTemplatePlugin()],
  // Use relative base path so assets work when served from any path prefix
  // The <base> tag in index.html will be set at runtime by the Go server
  base: './',
});
