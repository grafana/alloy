// panel.jsonnet defines utilities to create panels.

{
  new(title='', type=''):: {
    title: title,
    type: type,
    datasource: '${datasource}',
  },

  newSingleStat(title=''):: $.new(title, 'stat') {
    options: {
      colorMode: 'none',
      graphMode: 'none',
    },
  },

  newGraphedSingleStat(title=''):: $.new(title, 'stat') {
    pluginVersion: '9.0.6',
    fieldConfig: {
      defaults: {
        color: {
          mode: 'continuous-RdYlGr',
        },
      },
    },
    options: {
      colorMode: 'value',
      graphMode: 'area',
      text: { valueSize: 80 },
    },
  },

  newHeatmap(title=''):: $.new(title, 'heatmap') {
    maxDataPoints: 30,
    options: {
      calculate: false,
      color: {
        exponent: 0.5,
        fill: 'dark-orange',
        mode: 'scheme',
        scale: 'exponential',
        scheme: 'Oranges',
        steps: 65,
      },
      exemplars: {
        color: 'rgba(255,0,255,0.7)',
      },
      filterValues: {
        le: 1e-9,
      },
      tooltip: {
        show: true,
        yHistogram: true,
      },
      yAxis: {
        unit: 's',
      },
    },
    pluginVersion: '9.0.6',
  },

  withMultiTooltip():: {
    options+: {
      tooltip+: { mode: 'multi' },
    },
  },

  withUnit(unit):: {
    fieldConfig+: {
      defaults+: {
        unit: unit,
      },
    },
  },

  withOverrides(overrides):: {
    fieldConfig+: {
      overrides: overrides,
    },
  },

  withMappings(mappings):: {
    fieldConfig+: {
      defaults+: {
        mappings: mappings,
      },
    },
  },

  withCenteredAxis():: {
    fieldConfig+: {
      defaults+: {
        custom+: {
          axisCenteredZero: true,
        },
      },
    },
  },

  withPosition(pos):: { gridPos: pos },
  withDescription(desc):: { description: desc },
  withOptions(options):: { options: options },
  withTransformations(transformations):: { transformations: transformations },

  withQueries(queries):: { targets: queries },

  newQuery(expr='', format=null, legendFormat='__auto'):: std.prune({
    datasource: '${datasource}',
    expr: expr,
    format: format,
    legendFormat: legendFormat,
    range: true,
    instant: false,
  }),

  newInstantQuery(expr='', format=null, legendFormat='__auto'):: std.prune(
    $.newQuery(expr, format, legendFormat) {
      range: false,
      instant: true,
    }
  ),

  newNamedInstantQuery(expr='', refId='', format=null, legendFormat='__auto'):: std.prune(
    $.newQuery(expr, format, legendFormat) {
      range: false,
      instant: true,
      refId: refId,
    }
  ),

  newRow(title='', x=0, y=0, w=24, h=1, collapsed=false):: 
    $.new(title, 'row') 
    + $.withPosition({x: x, y: y, w: w, h: h })
    + {collapsed: collapsed},
}
