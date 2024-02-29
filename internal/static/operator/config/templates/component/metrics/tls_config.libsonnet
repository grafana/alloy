local optionals = import 'ext/optionals.libsonnet';
local secrets = import 'ext/secrets.libsonnet';

local new_safe_tls_config = import './safe_tls_config.libsonnet';

// @param {string} namespace
// @param {TLSConfig} config
function(namespace, config) new_safe_tls_config(namespace, config.SafeTLSConfig) + {
  // Local configurations for ca_file, cert_file, and key_file take precedence
  // over the SafeTLSConfig. Check local settings first and then fall back
  // to the safe settings.

  local has_ca_file = std.objectHasAll(config, 'CAFile'),
  local has_cert_file = std.objectHasAll(config, 'CertFile'),
  local has_key_file = std.objectHasAll(config, 'KeyFile'),

  ca_file:
    local unsafe = if has_ca_file then optionals.string(config.CAFile) else null;
    if unsafe == null then super.ca_file else unsafe,

  cert_file:
    local unsafe = if has_cert_file then optionals.string(config.CertFile) else null;
    if unsafe == null then super.cert_file else unsafe,

  key_file:
    local unsafe = if has_key_file then optionals.string(config.KeyFile) else null;
    if unsafe == null then super.key_file else unsafe,
}
