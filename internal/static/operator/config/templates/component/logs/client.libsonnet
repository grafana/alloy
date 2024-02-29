local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';

local new_external_labels = import './external_labels.libsonnet';
local new_tls_config = import 'component/metrics/tls_config.libsonnet';

// Generates the content of a client object to send logs to Loki.
//
// @param {GrafanaAgent} agent
// @param {string} namespace - namespace of spec.
// @param {LogsClientSpec} spec
function(agent, namespace, spec) {
  url: spec.URL,
  tls_config:
    if spec.TLSConfig != null then new_tls_config(namespace, spec.TLSConfig),
  proxy_url: optionals.string(spec.ProxyURL),

  tenant_id: optionals.string(spec.TenantID),

  timeout: optionals.string(spec.Timeout),
  batchwait: optionals.string(spec.BatchWait),
  batchsize: optionals.number(spec.BatchSize),

  basic_auth: if spec.BasicAuth != null then {
    username: secrets.valueForSecret(namespace, spec.BasicAuth.Username),
    password: secrets.valueForSecret(namespace, spec.BasicAuth.Password),
  },
  oauth2: (
    if spec.OAuth2 != null then {
      // TODO: client_id can also be stored in a config map:
      // secrets.valueForConfigMap(namespace, spec.OAuth2.ClientID.ConfigMap),
      local client_id = secrets.valueForSecret(namespace, spec.OAuth2.ClientID.Secret),
      
      client_id: client_id,
      client_secret_file: secrets.pathForSecret(namespace, spec.OAuth2.ClientSecret),
      endpoint_params: spec.OAuth2.EndpointParams,
      scopes: spec.OAuth2.Scopes,
      token_url: spec.OAuth2.TokenURL,
    }
  ),
  bearer_token: optionals.string(spec.BearerToken),
  bearer_token_file: optionals.string(spec.BearerTokenFile),

  backoff_config: if spec.BackoffConfig != null then {
    min_period: optionals.string(spec.BackoffConfig.MinPeriod),
    max_period: optionals.string(spec.BackoffConfig.MaxPeriod),
    max_retries: optionals.number(spec.BackoffConfig.MaxRetries),
  },

  external_labels: optionals.object(new_external_labels(agent, spec)),
}
