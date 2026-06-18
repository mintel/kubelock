# Init Container Example

* Start minikube and set your docker client to point to the daemon inside minikube:
  ```bash
  minikube start
  eval $(minikube docker-env)
  ```
* Build the example docker image inside minikube. This pulls a standard Hello World nginx example website and copies in the kubelock binary and the script we want to run in the init container:
  ```
  docker build -t kubelock-example .
  ```
* Create the namespace and apply the manifests to your cluster:
  ```
  kubectl create namespace kubelock
  kubectl apply -k kustomize
  ```

This will start 3 Pods whose initContainers will all race to acquire a Lease named `example-app`. Whichever container gets there first becomes the holder, with its identity set to the Pod hostname, then continues to refresh the lease while the wrapped command (in this case, `sh db-migrations.sh`) is running. During this time the other Pods will retry every _`--retry-period`_ seconds. Once the wrapped command completes, the first container releases the lease and the next to acquire it runs its command. This continues until all containers have completed their task.
