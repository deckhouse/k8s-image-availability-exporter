package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/flant/k8s-image-availability-exporter/pkg/handlers"
	"github.com/flant/k8s-image-availability-exporter/pkg/logging"
	"github.com/flant/k8s-image-availability-exporter/pkg/registry"

	"github.com/google/go-containerregistry/pkg/name"

	"github.com/sirupsen/logrus"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/sample-controller/pkg/signals"
	_ "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func main() {
	imageCheckInterval := flag.Duration("check-interval", time.Minute, "image re-check interval")
	ignoredImagesStr := flag.String("ignored-images", "", "tilde-separated image regexes to ignore, each image will be checked against this list of regexes")
	bindAddr := flag.String("bind-address", ":8080", "address:port to bind /metrics endpoint to")
	namespaceLabels := flag.String("namespace-label", "", "namespace label for checks")
	insecureSkipVerify := flag.Bool("skip-registry-cert-verification", false, "whether to skip registries' certificate verification")
	plainHTTP := flag.Bool("allow-plain-http", false, "whether to fallback to HTTP scheme for registries that don't support HTTPS") // named after the ctr cli flag
	defaultRegistry := flag.String("default-registry", "", fmt.Sprintf("default registry to use in absence of a fully qualified image name, defaults to %q", name.DefaultRegistry))

	flag.Parse()

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.AddHook(logging.NewPrometheusHook())

	// set up signals, so we handle the first shutdown signal gracefully
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

	var regexes []regexp.Regexp
	if *ignoredImagesStr != "" {
		regexStrings := strings.Split(*ignoredImagesStr, "~")
		for _, regexStr := range regexStrings {
			regexes = append(regexes, *regexp.MustCompile(regexStr))
		}
	}

	registryChecker := registry.NewChecker(
		stopCh,
		kubeClient,
		*insecureSkipVerify,
		*plainHTTP,
		regexes,
		*defaultRegistry,
		*namespaceLabels,
	)
	prometheus.MustRegister(registryChecker)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", handlers.Healthz)
	go func() {
		log.Fatal(http.ListenAndServe(*bindAddr, nil))
	}()

	handlers.UpdateHealth(true)

	wait.Until(func() {
		registryChecker.Tick()
		liveTicksCounter.Inc()
	}, *imageCheckInterval, stopCh)
}
