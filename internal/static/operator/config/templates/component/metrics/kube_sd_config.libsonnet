local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';
local k8s = import 'utils/k8s.libsonnet';

local new_tls_config = import './tls_config.libsonnet';

// kubernetes_sd_config returns a kubernetes_sd_config entry.
//
// @param {string} namespace - Namespace of GrafanaAgent resource
// @param {string[]} namespaces - Namespaces to discover resources in
// @param {APIServerConfig} apiServer - config to use for k8s discovery.
// @param {string} role - role of k8s resources to discover.
function(
  namespace,
  namespaces,
  apiServer,
  role,
) {
  role: role,
  namespaces: if std.length(k8s.array(namespaces)) > 0 then {
    names: namespaces,
  },

  api_server: if apiServer != null then optionals.string(apiServer.Host),

  basic_auth: if apiServer != null && apiServer.BasicAuth != null then {
    username: secrets.valueForSecret(namespace, apiServer.BasicAuth.Username),
    password: secrets.valueForSecret(namespace, apiServer.BasicAuth.Password),
  },

  local bearerToken = if apiServer != null then optionals.string(apiServer.BearerToken),
  local bearerTokenFile = if apiServer != null then optionals.string(apiServer.BearerTokenFile),

  authorization: if bearerToken != null || bearerTokenFile != null then {
    type: 'Bearer',
    credentials: bearerToken,
    credentials_file: bearerTokenFile,
  },

  tls_config: if apiServer != null && apiServer.TLSConfig != null then
    new_tls_config(namespace, apiServer.TLSConfig),
}
