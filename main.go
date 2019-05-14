package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type Addon struct {
	Channel string `json:"channel"`
	Version string `json:"version"`
	Id      string `json:"id"`
}

var (
	home            = homedir.HomeDir()
	addr            = flag.String("listen-address", ":9598", "the address to listen on for HTTP requests.")
	kubeconfig      = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	enableInCluster = flag.Bool("in-cluster", true, "enable in cluster configuration")

	updatedTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "kops_channel_updated",
			Help: "Last time the addon was updated.",
		},
		[]string{"name", "channel", "version", "id"},
	)
)

const (
	systemNamespace = "kube-system"
	addonPrefix     = "addons.k8s.io/"

	// TODO(tvi): Make it configurable
	refreshPeriod = 15 * time.Second
)

func init() {
	prometheus.MustRegister(updatedTimestamp)
}

// TODO(tvi): Add context timeout
// TODO(tvi): Handle error cases better
func fetch() {
	flag.Parse()

	var config *rest.Config
	var err error
	if *enableInCluster {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err)
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err)
		}
	}

	c, err := v1.NewForConfig(config)
	if err != nil {
		panic(err)
	}
	n, err := c.Namespaces().Get(systemNamespace, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}
	for name, rawJson := range n.ObjectMeta.Annotations {
		if strings.HasPrefix(name, addonPrefix) {
			a := &Addon{}
			if err := json.Unmarshal([]byte(rawJson), a); err != nil {
				panic(err)
			}

			now := int32(time.Now().Unix())
			updatedTimestamp.With(prometheus.Labels{
				"name":    name,
				"channel": a.Channel,
				"version": a.Version,
				"id":      a.Id,
			}).Set(float64(now))
		}
	}
}

func main() {
	go func() {
		for {
			fetch()
			time.Sleep(refreshPeriod)
		}
	}()

	flag.Parse()
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "OK\n")
	})
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Listening on %v\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
