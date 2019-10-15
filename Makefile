SHELL := /bin/bash

BINARY_NAME ?= kubelock
DOCKER_REGISTRY ?= mintel
DOCKER_IMAGE = ${DOCKER_REGISTRY}/${BINARY_NAME}

VERSION ?= $(shell echo `git symbolic-ref -q --short HEAD || git describe --tags --exact-match` | tr '[/]' '-')
DOCKER_TAG ?= ${VERSION}

go-build: kubelock

docker:
	docker build -t ${DOCKER_IMAGE}:${DOCKER_TAG} .

docker-ci:
	docker build -t mintel/kubelock:ci --target=builder .

docker-minikube: minikube-check
	@echo "building docker image"
	@eval $$(minikube docker-env); \
	docker build -t kubelock-example -f Dockerfile-dev .

minikube-check:
	@echo "checking minikube"
	@minikube status || (echo "ERROR: minikube is not ready"; exit 1)
	@if [[ ! $$(kubectl config current-context) == "minikube" ]]; then \
		echo "ERROR: wrong kubectl context"; \
		exit 1; \
	fi

clean: minikube-check
	@echo "cleaning kubelock namespace"
	@if [[ $$(kubectl get ns kubelock 2>/dev/null) ]]; then \
		kubectl delete ns kubelock >/dev/null; \
	fi

minikube: minikube-check clean docker-minikube
	@echo "applying manifests"
	@kubectl create ns kubelock >/dev/null; \
	kubectl apply -k examples/init-container/kustomize >/dev/null

kubelock : main.go
	@echo "building go binary"
	@GOOS=linux go build -o ./kubelock .

test : minikube-check
	@go test -cover
