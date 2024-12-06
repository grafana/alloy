
echo "running producer $1 on port $2"
INSTANCE=$1 MIMIR_PWD_FILE=~/workspace/go-playground/secrets/MIMIR_TOKEN alloy run --stability.level experimental producer.alloy --server.http.listen-addr "127.0.0.1:$2" --storage.path "./tmp/data_$1"
