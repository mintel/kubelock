package main

import (
	"context"
	"flag"
	"fmt"
	"k8s.io/client-go/tools/leaderelection"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"testing"
	"time"
)

var defaultKubeConfig = fmt.Sprintf("%s/.kube/config", os.Getenv("HOME"))

func helperCommandContext(t *testing.T, ctx context.Context, s ...string) (cmd *exec.Cmd) {
	cs := []string{"-test.run=TestHelperProcess", "--"}
	cs = append(cs, s...)
	if ctx != nil {
		cmd = exec.CommandContext(ctx, os.Args[0], cs...)
	} else {
		cmd = exec.Command(os.Args[0], cs...)
	}
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func helperCommand(t *testing.T, s ...string) *exec.Cmd {
	return helperCommandContext(t, nil, s...)
}

func TestProcessArgs(t *testing.T) {
	flag.Set("v", "10")
	var tests = []struct {
		Name        string
		args        []string
		errExpected bool
	}{
		{"no-args", make([]string, 0), true},
		{"one-arg", []string{"sleep"}, false},
		{"two-args", []string{"sleep", "10"}, false},
	}
	for _, test := range tests {
		if err := processArgs(test.args); err == nil && test.errExpected {
			t.Errorf("%s: Expected error, got no error", test.Name)
		}
		if err := processArgs(test.args); err != nil && !test.errExpected {
			t.Errorf("%v: Expected no error, got error: %v", test.Name, err)
		}
	}
}

func TestKubeSetup(t *testing.T) {
	var tests = []struct {
		Name        string
		kubeconfig  string
		errExpected bool
	}{
		{"fake-config", "/fake/file/path", true},
		{"real-config", defaultKubeConfig, false},
		// TODO: "no-config" (in-cluster)
	}
	for _, test := range tests {
		if _, err := kubeSetup(test.kubeconfig); err == nil && test.errExpected {
			t.Errorf("%s: Expected error, got no error", test.Name)
		}
		if _, err := kubeSetup(test.kubeconfig); err != nil && !test.errExpected {
			t.Errorf("%s: Expected no error, got error: %v", test.Name, err)
		}
	}
}

func setupLeaderElect(t *testing.T, err *error, args ...string) (leaderelection.LeaderCallbacks, context.Context) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := helperCommandContext(t, ctx, args...)
	cb := getLeaderCallbacks(cmd, err, ctx, cancel)
	return cb, ctx
}

func TestLeaderElect(t *testing.T) {
	var subprocessErr error
	var cb leaderelection.LeaderCallbacks
	var ctx context.Context

	// Successful run
	cb, ctx = setupLeaderElect(t, &subprocessErr, "sleep", "3")
	cb.OnStartedLeading(ctx)
	if subprocessErr != nil {
		t.Errorf("Expected no error, got %v:", subprocessErr)
	}

	// Exit with returncode 12
	exp := 12
	cb, ctx = setupLeaderElect(t, &subprocessErr, "exit", strconv.Itoa(exp))
	cb.OnStartedLeading(ctx)
	if subprocessErr == nil {
		t.Errorf("Expected error, didn't get one")
	} else {
		if e, ok := subprocessErr.(*exec.ExitError); ok {
			if act := e.ExitCode(); act != exp {
				t.Errorf("Expected exitcode %v, got %v", strconv.Itoa(exp), strconv.Itoa(act))
			}
		} else {
			t.Errorf("Expected ExitError, got %v", reflect.TypeOf(subprocessErr))
		}
	}

}

func TestSoftKill(t *testing.T) {
	var err error
	ctx := context.Background()
	cmd := helperCommandContext(t, ctx, "sleep", "10")
	err = cmd.Start()
	t.Logf("err: %v", err)
	time.Sleep(2 * time.Second)
	_, err = softKill(cmd, os.Interrupt, 1)
	t.Logf("err: %v", err)
	err = cmd.Wait()
	t.Logf("err: %v", err)
}

// TestHelperProcess isn't a real test. It's used as a helper process
func TestHelperProcess(*testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]
	switch cmd {
	case "echo":
		iargs := []interface{}{}
		for _, s := range args {
			iargs = append(iargs, s)
		}
		fmt.Println(iargs...)
	case "exit":
		n, _ := strconv.Atoi(args[0])
		os.Exit(n)
	case "sleep":
		n, _ := strconv.Atoi(args[0])
		time.Sleep(time.Duration(n) * time.Second)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n", cmd)
		os.Exit(2)
	}
}
