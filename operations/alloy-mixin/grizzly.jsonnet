// grizzly.jsonnet allows you to test this mixin using Grizzly.
//
// To test, first set the GRAFANA_URL environment variable to the URL of a
// Grafana instance to deploy the mixin (i.e., "http://localhost:3000").
//
// Then, run `grr watch . ./grizzly.jsonnet` from this directory to watch the
// mixin and continually deploy all dashboards.
//
// By default, only dashboards get deployed; not alerts or recording rules.
// To deploy alerts and recording rules, set up the environment variables used
// by cortextool to authenticate with a Prometheus or Alertmanager intance.

(import './grizzly/dashboards.jsonnet') +
(import './grizzly/alerts.jsonnet')
