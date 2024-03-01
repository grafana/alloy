#!/usr/bin/env bash
mkdir ./tutorials
cd ./tutorials || exit
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/docker-compose.yaml -o ./docker-compose.yaml
mkdir -p ./mimir
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/mimir/mimir.yaml -o ./mimir/mimir.yaml
mkdir -p ./flow_configs
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/agent.flow -o ./flow_configs/agent.flow
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/example.flow -o ./flow_configs/example.flow
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/relabel.river -o ./flow_configs/relabel.river
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/multiple-inputs.river -o ./flow_configs/multiple-inputs.river
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/README.md -o ./flow_configs/README.md
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/relabel.flow -o ./flow_configs/relabel.flow
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/multiple-inputs.flow -o ./flow_configs/multiple-inputs.flow
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/agent.river -o ./flow_configs/agent.river
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/flow_configs/example.river -o ./flow_configs/example.river
mkdir -p ./grafana
mkdir -p ./grafana/datasources
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/grafana/datasources/datasource.yml -o ./grafana/datasources/datasource.yml
mkdir -p ./grafana/dashboards-provisioning
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/grafana/dashboards-provisioning/dashboards.yaml -o ./grafana/dashboards-provisioning/dashboards.yaml
mkdir -p ./grafana/config
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/grafana/config/grafana.ini -o ./grafana/config/grafana.ini
mkdir -p ./grafana/dashboards
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/grafana/dashboards/template.jsonnet -o ./grafana/dashboards/template.jsonnet
curl https://raw.githubusercontent.com/grafana/agent/main/docs/sources/flow/tutorials/assets/grafana/dashboards/agent.json -o ./grafana/dashboards/agent.json
docker pull grafana/agent:main 
CONFIG_FILE=$1 docker-compose -f ./docker-compose.yaml up
