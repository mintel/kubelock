#!/usr/bin/env bash

BINARY_NAME="kubelock"
DOCKER_REGISTRY="mintel"
KIND_CONTEXT="kubernetes-admin@kind"
TEST_REPLICAS=3

THISDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

function get_annotation {
  kubectl -n kubelock get endpoints example-app -o json | jq -r '.metadata.annotations."control-plane.alpha.kubernetes.io/leader"'
}

function refresh_annotation_vars {
  annotation=$(get_annotation)
  holderIdentity=$(echo "${annotation}" | jq -r .holderIdentity)
  leaseDurationSeconds=$(echo "${annotation}" | jq -r .leaseDurationSeconds)
  acquireTime=$(echo "${annotation}" | jq -r .acquireTime)
  renewTime=$(echo "${annotation}" | jq -r .renewTime)
  leaderTransitions=$(echo "${annotation}" | jq -r .leaderTransitions)
}

# Check that kubectl is configured with the right context

if [[ ! $(kubectl config current-context) == "${KIND_CONTEXT}" ]]; then
  echo "ERROR: wrong kubectl context"
	exit 1
else
  echo "INFO: kind kubectl context configured"
fi

# Create a namespace if necessary

if ! kubectl get namespace ${BINARY_NAME} > /dev/null 2>&1; then
  echo "INFO: creating ${BINARY_NAME} namespace"
  kubectl create namespace ${BINARY_NAME}
else
  echo "INFO: ${BINARY_NAME} namespace exists"
fi

# Load the docker image into kind

#kind load docker-image ${DOCKER_REGISTRY}/${BINARY_NAME}:ci --loglevel debug

# Apply the manifests

kubectl apply -f "${THISDIR}"/manifests

# Wait for endpoint annotation
annotation=$(get_annotation)
while [[ $annotation == "null" ]]; do
  echo "INFO: waiting for annotation"
  sleep 1
  annotation=$(get_annotation)
done

function check_terminated {
  local statuses && statuses=$1
  local maxTerminated && maxTerminated=$2
  terminated=0
  for status in $statuses; do
    if [[ $status == 'terminated' ]]; then
      terminated++
    fi
  done
  if [[ $terminated -gt $maxTerminated ]]; then
    return 1
  else
    return 0
  fi
}

refresh_annotation_vars
while true; do

  echo "holderIdentity: $holderIdentity"
  echo "leaseDurationSeconds: $leaseDurationSeconds"
  echo "acquireTime: $acquireTime"
  echo "renewTime: $renewTime"
  echo "leaderTransitions: $leaderTransitions"

  if [[ $holderIdentity != "" ]]; then
    while [[ $holderStatus != "terminated" ]]; do
      getPods=$(kubectl -n kubelock get pods -o json)
      holderStatus=$(echo "$getPods" | jq -r '.items[] | select(.metadata.name | contains("'"$holderIdentity"'")) | .status.initContainerStatuses[].state | keys[0]')
      echo "holderStatus: $holderStatus"
      allStatuses=$(echo "$getPods" | jq -r '.items[] | .status.initContainerStatuses[].state | keys[0]')
      echo "allStatuses: $allStatuses"
      sleep 1
    done
    holderStatus=""
  fi

  echo "---"

  if [[ $leaderTransitions -eq $TEST_REPLICAS ]]; then
    break
  fi

  sleep 1
  refresh_annotation_vars

done

# Clean up

kubectl delete namespace kubelock