#!/usr/bin/env bash

BINARY_NAME="kubelock"
KIND_CONTEXT="kubernetes-admin@kind"
TEST_REPLICAS=3
EXITSTATUS=0

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

# Apply the manifests

kubectl apply -f "${THISDIR}"/manifests

# Wait for endpoint annotation

annotation=$(get_annotation)
echo -n "INFO: waiting for annotation"
while [[ $annotation == "null" ]]; do
  echo -n "."
  sleep 1
  annotation=$(get_annotation)
done
echo

function check_terminated {
  local statuses && statuses=$1
  local expected && expected=$2
  terminated=0
  for status in $statuses; do
    if [[ $status == 'terminated' ]]; then
      ((terminated++))
    fi
  done
  if [[ $terminated != "$expected" ]]; then
    return 1
  else
    return 0
  fi
}

jobsCompleted=0
refresh_annotation_vars
while true; do

  echo "DEBUG: holderIdentity: $holderIdentity"
  echo "DEBUG: leaseDurationSeconds: $leaseDurationSeconds"
  echo "DEBUG: acquireTime: $acquireTime"
  echo "DEBUG: renewTime: $renewTime"
  echo "DEBUG: leaderTransitions: $leaderTransitions"

  if [[ $holderIdentity != "" ]]; then
    echo "INFO: lock obtained"

    while [[ $holderStatus != "terminated" ]]; do
      getPods=$(kubectl -n kubelock get pods -o json)
      holderStatus=$(echo "$getPods" | jq -r '.items[] | select(.metadata.name | contains("'"$holderIdentity"'")) | .status.initContainerStatuses[].state | keys[0]')
      #echo "DEBUG: holderStatus: $holderStatus"
      allStatuses=$(echo "$getPods" | jq -r '.items[] | .status.initContainerStatuses[].state | keys[0]')
      #echo "DEBUG: allStatuses: $allStatuses"
      sleep 1
    done

    ((jobsCompleted++))

    echo "=== RUN   TestTerminatedPods: $jobsCompleted/$TEST_REPLICAS"
    if ! check_terminated "$allStatuses" "$leaderTransitions"; then
      echo "FAIL: expected allStatuses: $leaderTransitions, leaderTransitions: $leaderTransitions, got allStatuses: $allStatuses, leaderTransitions: $leaderTransitions"
      EXITSTATUS=1
    else
      echo "--- PASS: TestTerminatedPods: $jobsCompleted/$TEST_REPLICAS"
    fi

    holderStatus=""
  fi

  if [[ $leaderTransitions -eq $TEST_REPLICAS ]]; then
    break
  fi

  sleep 1
  refresh_annotation_vars

done

# Clean up

kubectl delete namespace kubelock

exit $EXITSTATUS
