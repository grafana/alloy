// grizzly.jsonnet allows you to test this mixin using Grizzly.
//
// To test, first set the GRAFANA_URL environment variable to the URL of a
// Grafana instance to deploy the mixin (i.e., "http://localhost:3000").
//
// Then, run `grr watch . ./grizzly.jsonnet` from this directory to watch the
// mixin and continually deploy all dashboards.
//

(import './grizzly/dashboards.jsonnet')

// By default, alerts get also deployed; This should work out-of-the-box when
// using the example docker-compose environment. If you are using grizzly with
// a different environemnt, set up the environment variables as documented in
// https://grafana.github.io/grizzly/configuration/ or comment out the line below.
+ (import './grizzly/alerts.jsonnet')
