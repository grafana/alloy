local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';

local new_relabel_config = import './relabel_config.libsonnet';
local new_tls_config = import './tls_config.libsonnet';

// Generates the contents of a remote_write object.
//
// @param {string} namespace - namespace of the RemoteWriteSpec.
// @param {RemoteWriteSpec} rw
function(namespace, rw) {
  // TODO(rfratto): follow_redirects
  // TODO(rfratto): retry_on_http_429, currently experimental

  url: rw.URL,
  name: optionals.string(rw.Name),
  remote_timeout: optionals.string(rw.RemoteTimeout),
  headers: optionals.object(rw.Headers),
  proxy_url: optionals.string(rw.ProxyURL),

  write_relabel_configs: optionals.array(std.map(
    new_relabel_config,
    rw.WriteRelabelConfigs,
  )),

  tls_config: (
    if rw.TLSConfig != null then
      new_tls_config(namespace, rw.TLSConfig)
  ),

  basic_auth: (
    if rw.BasicAuth != null then {
      username: secrets.valueForSecret(namespace, rw.BasicAuth.Username),
      password_file: secrets.pathForSecret(namespace, rw.BasicAuth.Password),
    }
  ),
  oauth2: (
    if rw.OAuth2 != null then {
      // TODO: client_id can also be stored in a config map:
      // secrets.valueForConfigMap(namespace, rw.OAuth2.ClientID.ConfigMap),
      local client_id = secrets.valueForSecret(namespace, rw.OAuth2.ClientID.Secret),
      
      client_id: client_id,
      client_secret_file: secrets.pathForSecret(namespace, rw.OAuth2.ClientSecret),
      endpoint_params: rw.OAuth2.EndpointParams,
      scopes: rw.OAuth2.Scopes,
      token_url: rw.OAuth2.TokenURL,
    }
  ),
  local bearerToken = optionals.string(rw.BearerToken),
  local bearerTokenFile = optionals.string(rw.BearerTokenFile),

  authorization: if bearerToken != null || bearerTokenFile != null then {
    type: 'Bearer',
    credentials: bearerToken,
    credentials_file: bearerTokenFile,
  },

  sigv4: (
    if rw.SigV4 != null then {
      region: optionals.string(rw.SigV4.Region),
      profile: optionals.string(rw.SigV4.Profile),
      role_arn: optionals.string(rw.SigV4.RoleARN),
      access_key: secrets.valueForSecret(namespace, rw.SigV4.AccessKey),
      secret_key: secrets.valueForSecret(namespace, rw.SigV4.SecretKey),
    }
  ),

  queue_config: (
    if rw.QueueConfig != null then {
      capacity: optionals.number(rw.QueueConfig.Capacity),
      max_shards: optionals.number(rw.QueueConfig.MaxShards),
      min_shards: optionals.number(rw.QueueConfig.MinShards),
      max_samples_per_send: optionals.number(rw.QueueConfig.MaxSamplesPerSend),
      batch_send_deadline: optionals.string(rw.QueueConfig.BatchSendDeadline),
      min_backoff: optionals.string(rw.QueueConfig.MinBackoff),
      max_backoff: optionals.string(rw.QueueConfig.MaxBackoff),
    }
  ),

  metadata_config: (
    if rw.MetadataConfig != null then {
      send: rw.MetadataConfig.Send,
      send_interval: optionals.string(rw.MetadataConfig.SendInterval),
    }
  ),
}
