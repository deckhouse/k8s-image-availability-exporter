package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/flant/k8s-image-availability-exporter/pkg/cli"
	"github.com/flant/k8s-image-availability-exporter/pkg/handlers"
	"github.com/flant/k8s-image-availability-exporter/pkg/logging"
	"github.com/flant/k8s-image-availability-exporter/pkg/registry"
	"github.com/flant/k8s-image-availability-exporter/pkg/version"
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
	cp := newCaPaths()
	mirrors := newMirrorMap()
	forceCheckDisabledControllerKindsParser := cli.NewForceCheckDisabledControllerKindsParser()

	imageCheckInterval := flag.Duration("check-interval", time.Minute, "image re-check interval")
	ignoredImagesStr := flag.String("ignored-images", "", "tilde-separated image regexes to ignore, each image will be checked against this list of regexes")
	allowedImagesStr := flag.String("allowed-images", "", "tilde-separated image regexes to allow, each image will be checked against this list of regexes")
	bindAddr := flag.String("bind-address", ":8080", "address:port to bind /metrics endpoint to")
	namespaceLabels := flag.String("namespace-label", "", "namespace label for checks")
	insecureSkipVerify := flag.Bool("skip-registry-cert-verification", false, "whether to skip registries' certificate verification")
	plainHTTP := flag.Bool("allow-plain-http", false, "whether to fallback to HTTP scheme for registries that don't support HTTPS") // named after the ctr cli flag
	defaultRegistry := flag.String("default-registry", "", fmt.Sprintf("default registry to use in absence of a fully qualified image name, defaults to %q", name.DefaultRegistry))
	flag.Var(&cp, "capath", "path to a file that contains CA certificates in the PEM format") // named after the curl cli flag
	flag.Var(&mirrors, "image-mirror", "Add a mirror repository (format: original=mirror)")
	flag.Func("force-check-disabled-controllers", `comma-separated list of controller kinds for which image is forcibly checked, even when workloads are disabled or suspended. Acceptable values include "Deployment", "StatefulSet", "DaemonSet", "Cronjob" or "*" for all kinds (this option is case-insensitive)`, forceCheckDisabledControllerKindsParser.Parse)

	flag.Parse()

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.AddHook(logging.NewPrometheusHook())
	logrus.Infof("Starting k8s-image-availability-exporter %s", version.Version)

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

	var ignoredImgRegexes []regexp.Regexp
	if *ignoredImagesStr != "" {
		regexStrings := strings.Split(*ignoredImagesStr, "~")
		for _, regexStr := range regexStrings {
			ignoredImgRegexes = append(ignoredImgRegexes, *regexp.MustCompile(regexStr))
		}
	}

	var allowedImgRegexes []regexp.Regexp
	if *allowedImagesStr != "" {
		regexStrings := strings.Split(*allowedImagesStr, "~")
		for _, regexStr := range regexStrings {
			allowedImgRegexes = append(allowedImgRegexes, *regexp.MustCompile(regexStr))
		}
	}

	registryChecker := registry.NewChecker(
		stopCh.Done(),
		kubeClient,
		*insecureSkipVerify,
		*plainHTTP,
		cp,
		forceCheckDisabledControllerKindsParser.ParsedKinds,
		ignoredImgRegexes,
		allowedImgRegexes,
		*defaultRegistry,
		*namespaceLabels,
		mirrors,
	)
	prometheus.MustRegister(registryChecker)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/healthz", handlers.Healthz)
	go func() {
		logrus.Fatal(http.ListenAndServe(*bindAddr, nil))
	}()

	handlers.UpdateHealth(true)

	wait.Until(func() {
		registryChecker.Tick()
		liveTicksCounter.Inc()
	}, *imageCheckInterval, stopCh.Done())
}

/* Custom flag types */

var (
	_ flag.Value = (*caPaths)(nil)
	_ flag.Value = (*mirrorMap)(nil)
)

// caPaths is a custom flag type for a list of paths to CA certificates
type caPaths []string

func newCaPaths() caPaths {
	return caPaths{}
}

func (c *caPaths) String() string {
	return fmt.Sprintf("%v", *c)
}

func (c *caPaths) Set(value string) error {
	*c = append(*c, value)
	return nil
}

// mirrorMap is a custom flag type for a map of original to mirror repository names
type mirrorMap map[string]string

func newMirrorMap() mirrorMap {
	return make(mirrorMap)
}

func (m *mirrorMap) String() string {
	return fmt.Sprintf("%v", *m)
}

func (m *mirrorMap) Set(value string) error {
	result := strings.Split(value, "=")
	if len(result) != 2 {
		return errors.New("invalid format for mirror, must be original=mirror")
	}
	(*m)[result[0]] = result[1]
	return nil
}
