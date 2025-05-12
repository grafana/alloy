local build_image = import '../util/build_image.jsonnet';
local pipelines = import '../util/pipelines.jsonnet';
local secrets = import '../util/secrets.jsonnet';
local ghTokenFilename = '/drone/src/gh-token.txt';

// job_names gets the list of job names for use in depends_on.
local job_names = function(jobs) std.map(function(job) job.name, jobs);

local linux_containers = [
  { devel: 'alloy-devel', release: 'alloy' },
  { devel: 'alloy-devel-boringcrypto', release: 'alloy-boringcrypto' },
];

local linux_containers_jobs = std.map(function(container) (
  pipelines.linux('Publish Linux %s container' % container.release) {
    trigger: {
      ref: ['refs/tags/v*'],
    },
    steps: [{
      name: 'Fetch tags',
      image: build_image.linux,
	  commands: [
        'git fetch --tags',
	  ],
    }, {
      // We only need to run this once per machine, so it's OK if it fails. It
      // is also likely to fail when run in parallel on the same machine.
      name: 'Configure QEMU',
      image: build_image.linux,
      failure: 'ignore',
      volumes: [{
        name: 'docker',
        path: '/var/run/docker.sock',
      }],
      commands: [
        'docker run --rm --privileged multiarch/qemu-user-static --reset -p yes',
      ],
    }, {
      name: 'Publish container',
      image: build_image.linux,
      volumes: [{
        name: 'docker',
        path: '/var/run/docker.sock',
      }],
      environment: {
        DOCKER_LOGIN: secrets.docker_login.fromSecret,
        DOCKER_PASSWORD: secrets.docker_password.fromSecret,
        GCR_CREDS: secrets.gcr_admin.fromSecret,
      },
      commands: [
        'mkdir -p $HOME/.docker',
        'printenv GCR_CREDS > $HOME/.docker/config.json',
        'docker login -u $DOCKER_LOGIN -p $DOCKER_PASSWORD',

        // Create a buildx worker for our cross platform builds.
        'docker buildx create --name multiarch-alloy-%s-${DRONE_COMMIT_SHA} --driver docker-container --use' % container.release,

        './tools/ci/docker-containers %s' % container.release,

        'docker buildx rm multiarch-alloy-%s-${DRONE_COMMIT_SHA}' % container.release,
      ],
    }],
    volumes: [{
      name: 'docker',
      host: { path: '/var/run/docker.sock' },
    }],
  }
), linux_containers);


linux_containers_jobs + [

  pipelines.linux('Publish release') {
    trigger: {
      ref: ['refs/tags/v*'],
    },
    depends_on: job_names(linux_containers_jobs),
    image_pull_secrets: ['dockerconfigjson'],
    steps: [
      {
       name: 'Fetch tags',
       image: build_image.linux,
	   commands: [
        'git fetch --tags',
       ],
      },
      {
        name: 'Generate GitHub token',
        image: 'us.gcr.io/kubernetes-dev/github-app-secret-writer:latest',
        environment: {
          GITHUB_APP_ID: secrets.updater_app_id.fromSecret,
          GITHUB_APP_INSTALLATION_ID: secrets.updater_app_installation_id.fromSecret,
          GITHUB_APP_PRIVATE_KEY: secrets.updater_private_key.fromSecret,
        },
        commands: [
          '/usr/bin/github-app-external-token > %s' % ghTokenFilename,
        ],
      },
      {
        name: 'Publish release',
        image: build_image.linux,
        volumes: [{
          name: 'docker',
          path: '/var/run/docker.sock',
        }],
        environment: {
          DOCKER_LOGIN: secrets.docker_login.fromSecret,
          DOCKER_PASSWORD: secrets.docker_password.fromSecret,
          GPG_PRIVATE_KEY: secrets.gpg_private_key.fromSecret,
          GPG_PUBLIC_KEY: secrets.gpg_public_key.fromSecret,
          GPG_PASSPHRASE: secrets.gpg_passphrase.fromSecret,
        },
        commands: [
          'export GITHUB_TOKEN=$(cat %s)' % ghTokenFilename,
          'docker login -u $DOCKER_LOGIN -p $DOCKER_PASSWORD',
          'RELEASE_BUILD=1 VERSION="${DRONE_TAG}" make -j4 dist',
          |||
            VERSION="${DRONE_TAG}" RELEASE_DOC_TAG=$(echo "${DRONE_TAG}" | awk -F '.' '{print $1"."$2}') ./tools/release
          |||,
        ],
      },
    ],
    volumes: [{
      name: 'docker',
      host: { path: '/var/run/docker.sock' },
    }],
  },
]
