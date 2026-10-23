# Pushgateway endpoint example

Start the remote storage:

    docker compose -f example/pushgateway/docker-compose.yml up -d

Start Alloy (build first: `cd collector && go build -tags gore2regex -o ../build/alloy . && cd ..`):

    ./build/alloy run --storage.path=./example/pushgateway/data --server.http.listen-addr=127.0.0.1:12345 example/pushgateway/config.alloy

Push a metric:

    echo 'job_last_success_timestamp 1700000000' \
      | curl --data-binary @- http://127.0.0.1:9999/metrics/job/demo_batch/instance/worker-1

Read it back:

    curl -sG http://127.0.0.1:8428/api/v1/query --data-urlencode 'query=job_last_success_timestamp'

Alloy UI: http://127.0.0.1:12345 — vmsingle UI: http://127.0.0.1:8428/vmui
