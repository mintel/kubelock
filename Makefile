SHELL := /bin/bash

go-build: kubelock

docker-build: go-build minikube-check
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

minikube: go-build minikube-check clean docker-build
	@echo "applying manifests"
	@kubectl create ns kubelock >/dev/null; \
	kubectl apply -k examples/init-container/kustomize >/dev/null

kubelock : main.go
	@echo "building go binary"
	@GOOS=linux go build -o ./kubelock .

test : minikube-check
	@go test -cover
