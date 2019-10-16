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

This will start 3 Pods whose initContainers will all race to create a lock on the Endpoint created by the Service. Whichever container gets there first creates an Annotation (`control-plane.alpha.kubernetes.io/leader`), with its `holderIdentity` set to the Pod hostname, then continues to refresh its lease while the wrapped command (in this case, `sh db-migrations.sh`) is running. During this time the other Pods will test the lock every _`--retry-period`_ seconds. Once the wrapped command completes, the 1st container will release the lock and the next to test it will gain control. This continues until all containers have completed their task.
