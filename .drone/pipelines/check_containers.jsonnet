local build_image = import '../util/build_image.jsonnet';
local pipelines = import '../util/pipelines.jsonnet';

local linux_containers = [
  { name: 'grafana/alloy', make: 'make alloy-image', path: 'Dockerfile' },
];

(
  std.map(function(container) pipelines.linux('Check Linux container (%s)' % container.name) {
    trigger: {
      ref: ['refs/heads/main'],
      paths: [container.path, 'tools/ci/docker-containers'],
    },
    steps: [{
      name: 'Build container',
      image: build_image.linux,
      volumes: [{
        name: 'docker',
        path: '/var/run/docker.sock',
      }],
      commands: [container.make],
    }],
    volumes: [{
      name: 'docker',
      host: {
        path: '/var/run/docker.sock',
      },
    }],
  }, linux_containers)
)
