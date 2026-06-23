package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	dbcpcontroller "go-entry-task/cloud_native_entry_task/controllers/internal/controller"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	var kubeconfig string
	var namespace string
	var resync time.Duration

	flag.StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig for local development")
	flag.StringVar(&namespace, "namespace", os.Getenv("WATCH_NAMESPACE"), "namespace to watch; empty means all namespaces")
	flag.DurationVar(&resync, "resync", 5*time.Second, "full reconcile interval")
	flag.Parse()

	cfg, err := buildKubeConfig(kubeconfig)
	if err != nil {
		log.Fatalf("build kube config: %v", err)
	}
	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("create kubernetes client: %v", err)
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("create dynamic client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctrl := dbcpcontroller.New(kube, dyn, dbcpcontroller.Options{
		Namespace: namespace,
		Resync:    resync,
	})
	log.Printf("starting dbcp-entry-service controller namespace=%q resync=%s", namespace, resync)
	if err := ctrl.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("run controller: %v", err)
	}
}

func buildKubeConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		defaultPath := filepath.Join(home, ".kube", "config")
		if _, statErr := os.Stat(defaultPath); statErr == nil {
			return clientcmd.BuildConfigFromFlags("", defaultPath)
		}
	}
	return rest.InClusterConfig()
}
