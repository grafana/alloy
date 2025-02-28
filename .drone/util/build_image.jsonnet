{
  local version = std.extVar('BUILD_IMAGE_VERSION'),

  linux: 'grafana/alloy-build-image:%s' % version,
  windows: 'grafana/alloy-build-image:%s' % version,
  boringcrypto: 'grafana/alloy-build-image:%s-boringcrypto' % version,
}
