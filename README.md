# kubelock

This app uses Kubernetes' leader election functionality to create or remove a distributed lock within the cluster. It has been designed with Django database migrations in mind - we wanted to ensure that only one Pod within a Deployment performed database migrations and that the other Pods were prevented from starting until these were complete - but you can wrap it around any shell script or command you want to make thread-safe from other Pods:

`kubelock --name my-pointless-lock ls`

## Usage

```
Usage: kubelock [OPTIONS] COMMAND

A utility to create/remove locks on Endpoints in Kubernetes while running a shell command/script

Options:
  -id string
    	ID of instance vying for leadership (default is the pod's hostname)
  -kubeconfig string
    	Absolute path to the kubeconfig file. Only set this if running out-of-cluster, otherwise it will create a config using the rights of the Service Account assigned to the Pods. You'll need a Service Account with "get", "list", "create" and "update" permissions to Endpoints resources in this case.
  -lease-duration string
    	How long the lock is held before another client will take over (default "15s")
  -name string
    	The lock endpoint name (default "example-app")
  -namespace string
    	The lock endpoint namespace (default "default")
  -renew-deadline string
    	The duration that the acting master will retry refreshing leadership before giving up. (default "10s")
  -retry-period string
    	The duration clients should wait between attempts to obtain a lock (default "2s")
```

Normally you'd just copy the binary from the official image into yours, then you can use as you wish:

```Dockerfile
FROM whatever

COPY --from=mintel/kubelock:latest /usr/local/bin/kubelock /usr/local/bin/

# ... your stuff here
```

Check the `examples` folder for more info.

Note that while `kubelock` prevents race conditions in the wrapped command/script as far as the Kubernetes API allows (see [here](https://godoc.org/k8s.io/client-go/tools/leaderelection) for details), it knows nothing of what the command is doing. Therefore it's important that you, or the app you call, provide the logic for the checks to ensure the same thing doesn't run twice.

## Local Development

Install minikube and kubectl if you haven't already; start minikube.

### Out-of-Cluster

* Make your changes
* Run `make go-build`
* Start 2 or three terminals and run a command in each
  ```bash
  ./kubelock --kubeconfig ~/.kube/config --id 1 --name example-app --namespace default sleep 10
  ./kubelock --kubeconfig ~/.kube/config --id 1 --name example-app --namespace default sleep 10
  ./kubelock --kubeconfig ~/.kube/config --id 1 --name example-app --namespace default sleep 10
  ```
* This will start three processes which will all race to lock an `default/example-app` Endpoints object, sleep for 10 seconds then release the lock and exit.

### In-Cluster (minikube)

Once you've made your changes, just run `make minikube`. This will:

1. Build the go binary
1. Ensure your `kubectl` is pointing at the `minikube` context
1. Delete the `kubelock` namespace on minikube if it exists
1. Point your docker client at the daemon inside minikube
1. Build the docker image inside the minikube host
1. Create a `kubelock` namespace
1. Apply the kustomize config in the `examples/init-container/kustomize` folder

Use [stern](https://github.com/wercker/stern) for monitoring the logs. It's much easier.
