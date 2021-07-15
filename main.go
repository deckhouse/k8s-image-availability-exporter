package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/flant/k8s-image-availability-exporter/pkg/logging"

	"github.com/flant/k8s-image-availability-exporter/pkg/handlers"

	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/flant/k8s-image-availability-exporter/pkg/registry_checker"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/pkg/signals"
	_ "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func main() {
	imageCheckInterval := flag.Duration("check-interval", time.Minute, "image re-check interval")
	ignoredImagesStr := flag.String("ignored-images", "", "comma-separated image names to ignore")
	bindAddr := flag.String("bind-address", ":8080", "address:port to bind /metrics endpoint to")
	insecureSkipVerify := flag.Bool("skip-registry-cert-verification", false, "whether to skip registries' certificate verification")
	specificNamespace := flag.String("namespace", "", "inspect specific namespace instead of whole k8s cluster")
	flag.Parse()

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.AddHook(logging.NewPrometheusHook())

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		logrus.Fatalf("Couldn't get Kubernetes default config: %s", err)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	liveTicksCounter := prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "k8s_image_availability_exporter",
			Name:      "completed_rechecks_total",
			Help:      "Number of image rechecks completed.",
		},
	)
	prometheus.MustRegister(liveTicksCounter)

	registryChecker := registry_checker.NewRegistryChecker(
		kubeClient,
		*insecureSkipVerify,
		strings.Split(*ignoredImagesStr, ","),
		*specificNamespace,
	)
	prometheus.MustRegister(registryChecker)

	registryChecker.Run(stopCh)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", handlers.Healthz)
	go func() {
		log.Fatal(http.ListenAndServe(*bindAddr, nil))
	}()

	handlers.UpdateHealth(true)

	wait.Until(func() {
		registryChecker.Check()
		liveTicksCounter.Inc()
	}, *imageCheckInterval, stopCh)
}
