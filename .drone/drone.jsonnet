local pipelines = import './pipelines.jsonnet';

(import 'pipelines/check_containers.jsonnet') +
(import 'pipelines/publish.jsonnet') +
(import 'util/secrets.jsonnet').asList
