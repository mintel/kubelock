package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"k8s.io/client-go/transport"
	"k8s.io/klog"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
	"time"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var (
	args struct {
		cmdName       string
		cmdArgs       []string
		id            string
		kubeconfig    string
		leaseDuration time.Duration
		lockName      string
		lockNamespace string
		renewDeadline time.Duration
		retryPeriod   time.Duration
	}
	basename = path.Base(os.Args[0])
)

func init() {
	// The leaderelection library's logging is excessive so we use glog
	// as our logging library and push klog output to /dev/null
	klog.SetOutput(ioutil.Discard)

	flag.Usage = func() {
		fmt.Printf("Usage: %s [OPTIONS] COMMAND\n\n", basename)
		fmt.Printf("A utility to create/remove locks on Endpoints in Kubernetes while running a shell command/script\n\n")
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
	}
	flag.StringVar(&args.id, "id", os.Getenv("HOSTNAME"), "ID of instance vying for leadership")
	flag.StringVar(&args.kubeconfig, "kubeconfig", "", "Absolute path to the kubeconfig file")
	flag.StringVar(&args.lockName, "name", "example-app", "The lease lock endpoint name")
	flag.StringVar(&args.lockNamespace, "namespace", "default", "The lease lock endpoint namespace")
	flag.DurationVar(&args.leaseDuration, "lease-duration", time.Second*15,
		"How long the lock is held before another client will take over")
	flag.DurationVar(&args.renewDeadline, "renew-deadline", time.Second*10,
		"The duration that the acting master will retry refreshing leadership before giving up.")
	flag.DurationVar(&args.retryPeriod, "retry-period", time.Second*2,
		"The duration clients should wait between attempts to obtain a lock")
	if err := flag.Set("logtostderr", "true"); err != nil {
		panic("Cannot set logtostderr.")
	}
	if err := flag.Set("stderrthreshold", "INFO"); err != nil {
		panic("Cannot set stderrthreshold.")
	}
}

// processArgs performs the validation on the command line arguments passed to kubelock
func processArgs(flagArgs []string) error {
	switch len(flagArgs) {
	case 0:
		return errors.New("you must provide a command for kubelock to run\n")
	case 1:
		args.cmdName = flagArgs[0]
	default:
		args.cmdName = flagArgs[0]
		args.cmdArgs = flagArgs[1:]
	}

	if glog.V(1) {
		glog.Infof("cmdName: %s", args.cmdName)
		glog.Infof("cmdArgs: %+q", args.cmdArgs)
		glog.Infof("id: %s", args.id)
		glog.Infof("leaseDuration: %s", args.leaseDuration)
		glog.Infof("lockName: %s", args.lockName)
		glog.Infof("lockNamespace: %s", args.lockNamespace)
		glog.Infof("renewDeadline: %s", args.renewDeadline)
		glog.Infof("retryPeriod: %s", args.retryPeriod)
	}
	return nil
}

type KubeConfig struct {
	config        *rest.Config
	clientset     *clientset.Clientset
	endpointsLock *resourcelock.EndpointsLock
}

// buildConfig will build a config object for either running outside the cluster using the kubectl config
// (if kubeconfig is set), or in-cluster using the service account assigned to the pods running the binary.
func kubeSetup(kcPath string) (kc *KubeConfig, err error) {
	kc = new(KubeConfig)
	if kcPath != "" {
		kc.config, err = clientcmd.BuildConfigFromFlags("", kcPath)
		if err != nil {
			return nil, err
		}
	} else {
		kc.config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}

	// Create the Clientset - used for querying the various API endpoints
	kc.clientset = clientset.NewForConfigOrDie(kc.config)

	// Create a resourcelock Interface for Endpoints objects since we will be placing the lock
	// annotation on the Endpoints resource assigned to the Service.
	kc.endpointsLock = &resourcelock.EndpointsLock{
		EndpointsMeta: metav1.ObjectMeta{
			Name:      args.lockName,
			Namespace: args.lockNamespace,
		},
		Client: kc.clientset.CoreV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: args.id,
		},
	}

	return kc, err
}

// hardKill kills the subprocess and any child processes it spawned.
// https://medium.com/@felixge/killing-a-child-process-and-all-of-its-children-in-go-54079af94773
func hardKill(cmd *exec.Cmd) {
	glog.Infof("Proceeding with SIGKILL.")
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		_ = cmd.Process.Kill() // Fallback
	}
}

// softKill tries to handle a termination signal gently by passing the signal to
// the subprocess and returns a channel with a 90 sec timeout
func softKill(cmd *exec.Cmd, sig os.Signal, timeout time.Duration) (<-chan time.Time, error) {
	glog.Infof("Received termination, signaling shutdown")
	if err := cmd.Process.Signal(sig); err != nil {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch, errors.New(fmt.Sprintf("error passing %s sig to subprocess", sig))
	} else {
		return time.After(timeout), nil // Give the subprocess 90s to stop gracefully.
	}
}

func getLeaderCallbacks(cmd *exec.Cmd, spErr *error, ctx context.Context, cancel context.CancelFunc) leaderelection.LeaderCallbacks {
	var leaderCallbacks = leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			glog.Info("Lock obtained. Running command.")
			defer cancel() // Exit leaderelection loop and release lock when chSubProcess.

			// Catch termination signals so they can be passed to the subprocess. Depending
			// on the OS it's probably not possible to catch Kill, but just in case...
			chSignals := make(chan os.Signal, 1)
			signal.Notify(chSignals, os.Interrupt, os.Kill, syscall.SIGTERM)
			defer signal.Stop(chSignals)

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = os.Stdin
			// Create a new process group with the same ID as the PID of the called subprocess. This is so that
			// when we call the hardKill() function we kill any child processes and do not leave any zombies behind.
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

			// Start the subprocess
			chSubProcess := make(chan error, 1)
			go func() {
				defer close(chSubProcess)
				*spErr = cmd.Run()
			}()

			var err error
			var chTermTimeout <-chan time.Time
			var softKilled bool
			for {
				select {
				case <-chSubProcess:
					glog.V(1).Info("chSubProcess: subprocess finished")
					return
				//case <-ctx.Done():
				//	defer hardKill(cmd)
				//	panic("context has been cancelled")
				case s := <-chSignals:
					glog.V(1).Infof("kubelock received signal from OS: %v", s)
					if !softKilled {
						glog.V(1).Infof("attempting soft kill")
						chTermTimeout, err = softKill(cmd, s, 90 * time.Second)
						if err != nil {
							glog.Errorf("Error soft-killing process: %s", err)
						}
						softKilled = true
					} else {
						glog.V(1).Infof("soft kill already attempted, performing hard kill")
						hardKill(cmd)
					}
				case <-chTermTimeout:
					glog.V(1).Info("chTermTimeout: received SIGINT, SIGTERM or SIGKILL from OS")
					glog.Infof("Timed out wait for subprocess to exit, killing")
					hardKill(cmd)
				}
			}
		},
		OnStoppedLeading: func() {
			select {
			case <-ctx.Done():
				glog.Info("Lock released.")
			default:
				// The lock shouldn't be released until OnStartedLeading finishes
				// and the context is canceled.
				panic("Lock released.")
			}
		},
		OnNewLeader: func(identity string) {
			if identity == args.id {
				return
			}
			glog.Infof("Current leader is %v", identity)
		},
	}
	return leaderCallbacks
}

func main() {

	// Process options/arguments
	flag.Parse()
	if err := processArgs(flag.Args()); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", basename, err)
		flag.Usage()
		os.Exit(1)
	}

	// Get the config and lock objects required for the leaderelection
	kc, err := kubeSetup(args.kubeconfig)
	if err != nil {
		glog.Fatalf("Unable to setup kube config: %v", err)
	}
	// Create a context with a cancel function that we will pass to the locking function
	// (leaderelection.RunOrDie).
	ctx, cancel := context.WithCancel(context.Background())
	// Add a WrapperFunc to the connection config to prevent any more requests
	// being sent to the API server after the above context has been cancelled.
	kc.config.Wrap(transport.ContextCanceller(ctx, fmt.Errorf("the leader is shutting down")))

	cmd := exec.CommandContext(ctx, args.cmdName, args.cmdArgs...)
	// Set up a variable in which the callback can store the output of the cmd.Run()
	// so we can query it for non-zero return codes at the end.
	var subprocessErr error
	cb := getLeaderCallbacks(cmd, &subprocessErr, ctx, cancel)

	// Start the leaderelection loop
	glog.Infof("Attempting to get a lock on %s/%v", args.lockNamespace, args.lockName)
	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            kc.endpointsLock,
		LeaseDuration:   args.leaseDuration,
		RenewDeadline:   args.renewDeadline,
		RetryPeriod:     args.retryPeriod,
		Callbacks:       cb,
		ReleaseOnCancel: true,
	})

	if subprocessErr != nil {
		if e, ok := subprocessErr.(*exec.ExitError); ok {
			os.Exit(e.ExitCode())
		} else {
			glog.Exitf("Error starting subprocess: %s", subprocessErr.Error())
		}
	}

}
