// agent-logs.libsonnet is the entrypoint for rendering a Grafana Agent
// config file for logs based on the Operator custom resources.
//
// When writing an object, any field will null will be removed from the final
// YAML. This is useful as we don't want to always translate unfilled values
// from the custom resources to a field in the YAML.
//
// A series of helper methods to convert default values into null (so they can
// be trimmed) are in ./ext/optionals.libsonnet.
//
// When writing a new function, please document the expected types of the
// arguments.

local marshal = import 'ext/marshal.libsonnet';
local optionals = import 'ext/optionals.libsonnet';

local new_logs_instance = import './logs.libsonnet';

// @param {config.Deployment} ctx
function(ctx) marshal.YAML(optionals.trim({
  local spec = ctx.Agent.Spec,
  local logs = spec.Logs,
  local namespace = ctx.Agent.ObjectMeta.Namespace,

  server: {
    log_level: optionals.string(spec.LogLevel),
    log_format: optionals.string(spec.LogFormat),
  },

  logs: {
    positions_directory: '/var/lib/grafana-agent/data',
    configs: optionals.array(std.map(
      function(logs_inst) new_logs_instance(
        agent=ctx.Agent,
        global=logs,
        instance=logs_inst,
        apiServer=spec.APIServerConfig,
        ignoreNamespaceSelectors=logs.IgnoreNamespaceSelectors,
        enforcedNamespaceLabel=logs.EnforcedNamespaceLabel,
      ),
      ctx.Logs,
    )),
  },
}))
