// Generates an individual integration.
//
// @param {Integration} integration
function(integration)
  // integration.Spec.Config.Raw is a base64 JSON string holding the raw config
  // for the integration.
  local raw = integration.Spec.Config.Raw;
  if raw == null || std.length(raw) == 0 then {}
  else std.parseJson(std.base64Decode(raw))
