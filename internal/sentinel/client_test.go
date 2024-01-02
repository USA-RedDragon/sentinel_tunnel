package sentinel

import (
	"context"
	"fmt"
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

var (
	testenv env.Environment
)

// TestMain wraps the test suite with a test environment in Kubernetes (KinD).
func TestMain(m *testing.M) {
	testenv = env.New()
	runName := envconf.RandomName("test", 16)

	testenv.Setup(
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println("Setting up cluster named", runName)
			return ctx, nil
		},
		envfuncs.CreateCluster(kind.NewProvider(), runName),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println("Creating namespace", runName)
			return ctx, nil
		},
		envfuncs.CreateNamespace(runName),
		installRedis(runName),
		// portForward(runName),
	)

	testenv.Finish(
		// unforwardPorts(runName),
		uninstallRedis(runName),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println("Deleting namespace", runName)
			return ctx, nil
		},
		envfuncs.DeleteNamespace(runName),
		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			fmt.Println("Destroying cluster named", runName)
			return ctx, nil
		},
		envfuncs.DestroyCluster(runName),
	)

	// launch package tests
	os.Exit(testenv.Run(m))
}

// func portForward(name string) env.Func {
// 	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
// 		config, err := clientcmd.BuildConfigFromFlags("", cfg.KubeconfigFile())
// 		if err != nil {
// 			return ctx, err
// 		}
// 	}
// }

// func unforwardPorts(name string) env.Func {
// 	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
// 	}
// }

func installRedis(name string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		helmManager := helm.New(cfg.KubeconfigFile())

		fmt.Println("Adding Bitnami Chart Repo")

		if err := helmManager.RunRepo(
			helm.WithArgs([]string{
				"add",
				"bitnami",
				"https://charts.bitnami.com/bitnami",
			}...)); err != nil {
			return ctx, err
		}

		fmt.Println("Installing Redis")

		if err := helmManager.RunInstall(
			helm.WithName(name),
			helm.WithNamespace(name),
			helm.WithChart("bitnami/redis"),
			helm.WithWait(),
			helm.WithArgs([]string{
				"--set", "auth.enabled=true",
				"--set", "replica.replicaCount=3",
				"--set", "sentinel.enabled=true",
			}...)); err != nil {
			return ctx, err
		}

		fmt.Println("Redis installed")

		return ctx, nil
	}
}

func uninstallRedis(name string) env.Func {
	return func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		helmManager := helm.New(cfg.KubeconfigFile())

		fmt.Println("Uninstalling Redis")

		if err := helmManager.RunUninstall(
			helm.WithName(name),
			helm.WithNamespace(name),
		); err != nil {
			return ctx, err
		}

		fmt.Println("Redis uninstalled")

		return ctx, nil
	}
}

func TestClient(t *testing.T) {
	t.Parallel()

	// config := TunnellingConfiguration{
	// 	SentinelsAddressesList: []string{
	// 		fmt.Sprintf("localhost:%s", sentinel1Port),
	// 		fmt.Sprintf("localhost:%s", sentinel2Port),
	// 		fmt.Sprintf("localhost:%s", sentinel3Port),
	// 	},
	// 	Password:  "password",
	// 	Databases: []TunnellingDbConfig{{Name: "mymaster", LocalPort: "6379"}},
	// }

	// stClient, err := NewTunnellingClient(config)
	// if err != nil {
	// 	t.Fatal(err)
	// }

	// ctx, cancel := context.WithCancel(context.Background())
	// errChan := make(chan error)
	// go func() {
	// 	errChan <- stClient.ListenAndServe(ctx)
	// }()
	// defer cancel()

	// Do the test now that we're listening on the Redis port

	// select {
	// case err := <-errChan:
	// 	t.Fatal(err)
	// default:
	// }
}
