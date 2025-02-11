local pipelines = import './pipelines.jsonnet';

(import 'pipelines/check_containers.jsonnet') +
(import 'pipelines/publish.jsonnet') +
(import 'pipelines/test_packages.jsonnet') +
(import 'util/secrets.jsonnet').asList
