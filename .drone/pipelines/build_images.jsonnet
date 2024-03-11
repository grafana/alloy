local pipelines = import '../util/pipelines.jsonnet';
local secrets = import '../util/secrets.jsonnet';

local locals = {
  on_merge: {
    ref: ['refs/heads/main'],
    paths: { include: ['build-image/**'] },
  },
  on_build_image_tag: {
    event: ['tag'],
    ref: ['refs/tags/build-image/v*'],
  },
  docker_environment: {
    DOCKER_LOGIN: secrets.docker_login.fromSecret,
    DOCKER_PASSWORD: secrets.docker_password.fromSecret,
  },
};

[
  // TODO(rfratto): The following are temporarily diasbled as grafana/alloy
  // gets set up.

  /*
  pipelines.linux('Create Linux build image') {
    trigger: locals.on_build_image_tag,
    steps: [{
      name: 'Build',
      image: 'docker',
      volumes: [{
        name: 'docker',
        path: '/var/run/docker.sock',
      }],
      environment: locals.docker_environment,
      commands: [
        'export IMAGE_TAG=${DRONE_TAG##build-image/v}',
        'docker login -u $DOCKER_LOGIN -p $DOCKER_PASSWORD',
        'docker run --rm --privileged multiarch/qemu-user-static --reset -p yes',
        'docker buildx create --name multiarch --driver docker-container --use',
        'docker buildx build --build-arg="GO_RUNTIME=golang:1.22.1-bullseye" --push --platform linux/amd64,linux/arm64 -t grafana/agent-build-image:$IMAGE_TAG ./build-image',
      ],
    }],
    volumes: [{
      name: 'docker',
      host: { path: '/var/run/docker.sock' },
    }],
  },
  pipelines.linux('Create Linux build image for boringcrypto') {
    trigger: locals.on_build_image_tag,
    steps: [{
      name: 'Build',
      image: 'docker',
      volumes: [{
        name: 'docker',
        path: '/var/run/docker.sock',
      }],
      environment: locals.docker_environment,
      commands: [
        'export IMAGE_TAG=${DRONE_TAG##build-image/v}-boringcrypto',
        'docker login -u $DOCKER_LOGIN -p $DOCKER_PASSWORD',
        'docker run --rm --privileged multiarch/qemu-user-static --reset -p yes',
        'docker buildx create --name multiarch --driver docker-container --use',
        'docker buildx build --build-arg="GO_RUNTIME=mcr.microsoft.com/oss/go/microsoft/golang:1.22.1-bullseye" --push --platform linux/amd64,linux/arm64 -t grafana/agent-build-image:$IMAGE_TAG ./build-image',
      ],
    }],
    volumes: [{
      name: 'docker',
      host: { path: '/var/run/docker.sock' },
    }],
   },

  pipelines.windows('Create Windows build image') {
    trigger: locals.on_build_image_tag,
    steps: [{
      name: 'Build',
      image: 'docker:windowsservercore-1809',
      volumes: [{
        name: 'docker',
        path: '//./pipe/docker_engine/',
      }],
      environment: locals.docker_environment,
      commands: [
        // NOTE(rfratto): the variable syntax is parsed ahead of time by Drone,
        // and not by Windows (where the syntax obviously wouldn't work).
        '$IMAGE_TAG="${DRONE_TAG##build-image/v}-windows"',
        'docker login -u $Env:DOCKER_LOGIN -p $Env:DOCKER_PASSWORD',
        'docker build -t grafana/agent-build-image:$IMAGE_TAG ./build-image/windows',
        'docker push grafana/agent-build-image:$IMAGE_TAG',
      ],
    }],
    volumes: [{
      name: 'docker',
      host: { path: '//./pipe/docker_engine/' },
    }],
  },
  */
]
