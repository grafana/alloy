#!/usr/bin/env bash

set +x

# Find `ci` directories
for chart_file in $(find * -name Chart.yaml -print | sort); do
  # Find chart
  CHART_DIR=$(dirname ${chart_file})
  CHART_NAME=$(basename ${CHART_DIR})
  TEST_DIR="${CHART_DIR}/../../tests" # We should append "/${CHART_NAME}" if we ever have more charts here

  if [ -d "${CHART_DIR}/ci" ]; then
    # tests directory is outside of the `charts` folder
    rm -rf ${TEST_DIR}
    mkdir -p ${TEST_DIR}
    for FILE_PATH in $(find ${CHART_DIR}/ci -name "*-values.yaml" -type f); do
      FILENAME=$(basename ${FILE_PATH})
      TESTNAME=${FILENAME%-values.yaml}
      # Render chart
      if [[ "${TESTNAME}" == *"vertical"* ]]; then
        helm template --namespace default --kube-version 1.26 --debug ${CHART_NAME} ${CHART_DIR} -f ${FILE_PATH} --output-dir ${TEST_DIR}/${TESTNAME} --set '$chart_tests=true' --api-versions autoscaling.k8s.io/v1
      else
        helm template --namespace default --kube-version 1.26 --debug ${CHART_NAME} ${CHART_DIR} -f ${FILE_PATH} --output-dir ${TEST_DIR}/${TESTNAME} --set '$chart_tests=true'
      fi
    done
  fi
done

CURRENT_DIR_NAME=${PWD##*/}
HELM_DIR="operations/helm"

if [ "${CURRENT_DIR_NAME}" == "helm" ]
then
  HELM_DIR="."
fi
  yamllint --config-file=${HELM_DIR}/lintconf.yaml ${HELM_DIR}/tests
