# Grafana Alloy UI

The Alloy UI uses the following:

- node v24.4
- npm v11
- vite 7

## How it works

Normal, local development is done via `npm run dev` from this folder (internal/web/ui).

However, API responses are not currently mocked, so most of the UI will not show any data.

To fully test the UI, run `npm run build`, then run Alloy as normal. This will use the "no built-in assets" version.

You can also run Alloy _with_ built-in assets if you'd like to test it that way.

## About the Alloy web server

The Alloy web server has the job of ensuring that the UI is available at the specified UI prefix.

This is accomplished by Vite rewriting the `index.html` file during a production build to include a template string that the UI can replace when serving the `index.html` page.
