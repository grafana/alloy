local build_image = import '../util/build_image.jsonnet';
local pipelines = import '../util/pipelines.jsonnet';

[
  pipelines.linux('Lint') {
    trigger: {
      event: ['pull_request'],
    },
    steps: [{
      name: 'Lint',
      image: build_image.linux,
      commands: [
        'apt-get update -y && apt-get install -y libsystemd-dev',
        'make lint',
      ],
    }],
  },

  pipelines.linux('Test') {
    trigger: {
      event: ['pull_request'],
    },
    steps: [{
      name: 'Run Go tests',
      image: build_image.linux,

      commands: [
        'make GO_TAGS="nodocker" test',
      ],
    }],
  },

  pipelines.linux('Test (Full)') {
    trigger: {
      ref: ['refs/heads/main'],
    },
    steps: [{
      name: 'Run Go tests',
      image: build_image.linux,
      volumes: [{
        name: 'docker',
        path: '/var/run/docker.sock',
      }],

      commands: [
        // The operator tests require K8S_USE_DOCKER_NETWORK=1 to be set when
        // tests are being run inside of a Docker container so it can access the
        // created k3d cluster properly.
        'K8S_USE_DOCKER_NETWORK=1 make test',
      ],
    }],
    volumes: [{
      name: 'docker',
      host: {
        path: '/var/run/docker.sock',
      },
    }],
  },

  pipelines.windows('Test (Windows)') {
    trigger: {
      event: ['pull_request'],
    },
    steps: [{
      name: 'Run Go tests',
      image: build_image.windows,
      commands: [
        pipelines.windows_command('echo "=== Workaround for failing clone step ==="'),
        pipelines.windows_command('echo "DRONE_COMMIT=${DRONE_COMMIT}"'),
        pipelines.windows_command('rm -rf .git/'),
        pipelines.windows_command('git clone https://github.com/grafana/alloy.git'),
        'cd alloy',
        pipelines.windows_command('git checkout ${DRONE_COMMIT}'),
        pipelines.windows_command('echo "=== DONE Workaround for failing clone step ==="'),
        pipelines.windows_command('go test -tags="nodocker,nonetwork" ./...'),
      ],
    }],
  },
]
