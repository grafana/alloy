local marshal = import 'ext/marshal.libsonnet';
local optionals = import 'ext/optionals.libsonnet';

// Creates a new stage.
//
// @param {spec} PipelineStageSpec.
local new_stage = function(spec) {
  // spec.Docker :: *DockerStageSpec
  docker: if spec.Docker != null then {},

  // spec.CRI :: *CRIStageSpec
  cri: if spec.CRI != null then {},

  // spec.Regex :: *RegexStageSpec
  regex: if spec.Regex != null then {
    expression: spec.Regex.Expression,
    source: optionals.string(spec.Regex.Source),
  },

  // spec.JSON :: *JSONStageSpec
  json: if spec.JSON != null then {
    expressions: spec.JSON.Expressions,
    source: optionals.string(spec.JSON.Source),
  },

  // spec.Replace :: *ReplaceStageSpec
  replace: if spec.Replace != null then {
    expression: spec.Replace.Expression,
    source: optionals.string(spec.Replace.Source),
    replace: optionals.string(spec.Replace.Replace),
  },

  // spec.Template :: *TemplateStageSpec
  template: if spec.Template != null then {
    source: spec.Template.Source,
    template: spec.Template.Template,
  },

  // spec.Pack :: *PackStageSpec
  pack: if spec.Pack != null then {
    labels: optionals.array(spec.Pack.Labels),
    ingest_timestamp: optionals.bool(spec.Pack.IngestTimestamp),
  },

  // spec.Timestamp :: *TimestampStageSpec
  timestamp: if spec.Timestamp != null then {
    source: spec.Timestamp.Source,
    format: spec.Timestamp.Format,
    fallback_formats: optionals.array(spec.Timestamp.FallbackFormats),
    location: optionals.string(spec.Timestamp.Location),
    action_on_failure: optionals.string(spec.Timestamp.ActionOnFailure),
  },

  // spec.Output :: *OutputStageSpec
  output: if spec.Output != null then {
    source: spec.Output.Source,
  },

  // spec.LabelDrop :: []string
  labeldrop: optionals.array(spec.LabelDrop),

  // spec.LabelAllow :: []string
  labelallow: optionals.array(spec.LabelAllow),

  // spec.Labels :: map[string]*string
  labels: optionals.object(spec.Labels),

  // spec.Limit :: *LimitStageSpec
  limit: if spec.Limit != null then {
    rate: optionals.number(spec.Limit.Rate),
    burst: optionals.number(spec.Limit.Burst),
    drop: if spec.Limit.Drop != null then spec.Limit.Drop else false
  },

  // spec.Metrics :: map[string]MetricsStageSpec
  metrics: if spec.Metrics != null then optionals.object(std.mapWithKey(
    function(key, value) {
      local metricType = std.asciiLower(value.Type),
      type:
        if metricType == 'counter' then 'Counter'
        else if metricType == 'gauge' then 'Gauge'
        else if metricType == 'histogram' then 'Histogram'
        else value.Type,  // Promtail will complain but for now it's better to do this than crash

      description: optionals.string(value.Description),
      prefix: optionals.string(value.Prefix),
      source: optionals.string(value.Source),
      max_idle_duration: optionals.string(value.MaxIdleDuration),

      config: {
        match_all: optionals.bool(value.MatchAll),
        count_entry_bytes: optionals.bool(value.CountEntryBytes),
        value: optionals.string(value.Value),
        action: value.Action,
        buckets: if value.Buckets != null then optionals.array(std.map(
          function(bucket)
            local val = std.parseJson(bucket);
            assert std.isNumber(val) : 'bucket must be convertible to float';
            val,
          value.Buckets,
        )),
      },
    },
    spec.Metrics,
  )),

  multiline: if spec.Multiline != null then {
    firstline: spec.Multiline.FirstLine,
    max_wait_time: optionals.string(spec.Multiline.MaxWaitTime),
    max_lines: optionals.number(spec.Multiline.MaxLines),
  },

  // spec.Tenant :: *TenantStageSpec
  tenant: if spec.Tenant != null then {
    label: optionals.string(spec.Tenant.Label),
    source: optionals.string(spec.Tenant.Source),
    value: optionals.string(spec.Tenant.Value),
  },

  // spec.Match :: *MatchStageSpec
  match: if spec.Match != null then {
    selector: spec.Match.Selector,
    pipeline_name: optionals.string(spec.Match.PipelineName),
    action: optionals.string(spec.Match.Action),
    drop_counter_reason: optionals.string(spec.Match.DropCounterReason),
    stages: if spec.Match.Stages != '' then (
      std.map(
        function(stage) new_stage(stage),
        marshal.intoStages(spec.Match.Stages),
      )
    ),
  },

  // spec.Drop :: *DropStageSpec
  drop: if spec.Drop != null then {
    source: optionals.string(spec.Drop.Source),
    expression: optionals.string(spec.Drop.Expression),
    value: optionals.string(spec.Drop.Value),
    older_than: optionals.string(spec.Drop.OlderThan),
    longer_than: optionals.string(spec.Drop.LongerThan),
    drop_counter_reason: optionals.string(spec.Drop.DropCounterReason),
  },
};

new_stage
