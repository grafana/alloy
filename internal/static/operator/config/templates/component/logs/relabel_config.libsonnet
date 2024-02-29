local optionals = import 'ext/optionals.libsonnet';

// @param {RelabelConfig} cfg
function(cfg) {
  source_labels: optionals.array(cfg.SourceLabels),
  separator: optionals.string(cfg.Separator),
  regex: optionals.string(cfg.Regex),
  modulus: optionals.number(cfg.Modulus),
  target_label: optionals.string(cfg.TargetLabel),
  replacement: optionals.string(cfg.Replacement),
  action: optionals.string(cfg.Action),
}
