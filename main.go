package main

import (
	"flag"
	"log"
	"net/http"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	listeningAddress = flag.String("telemetry.address", ":9104", "Address on which to expose metrics.")
	metricsEndpoint  = flag.String("telemetry.endpoint", "/metrics", "Path under which to expose metrics.")
	addr             = flag.String("addr", "unix:///var/run/docker.sock", "Docker address to connect to")
	parent           = flag.String("parent", "/docker", "Parent cgroup")
	authUser         = flag.String("auth.user", "", "Username for basic auth.")
	authPass         = flag.String("auth.pass", "", "Password for basic auth. Enables basic auth if set.")
	labelString      = flag.String("labels", "", "A comma seperated list of docker labels to export for containers.")
)

type basicAuthHandler struct {
	handler  http.HandlerFunc
	user     string
	password string
}

func (h *basicAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, password, ok := r.BasicAuth()
	if !ok || password != h.password || user != h.user {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"metrics\"")
		http.Error(w, "Invalid username or password", http.StatusUnauthorized)
		return
	}
	h.handler(w, r)
	return
}

func main() {
	flag.Parse()
	manager := newDockerManager(*addr, *parent)
	var labels []string
	if *labelString != "" {
		labels = strings.Split(*labelString, ",")
	} else {
		labels = make([]string, 0)
	}

	dockerClient, err := docker.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		log.Fatalf("Unable to start docker client %v", err.Error())
	}
	exporter := NewExporter(manager, *dockerClient, labels)
	prometheus.MustRegister(exporter)

	log.Printf("Starting Server: %s", *listeningAddress)
	handler := prometheus.Handler()
	if *authUser != "" || *authPass != "" {
		if *authUser == "" || *authPass == "" {
			log.Fatal("You need to specify -auth.user and -auth.pass to enable basic auth")
		}
		handler = &basicAuthHandler{
			handler:  prometheus.Handler().ServeHTTP,
			user:     *authUser,
			password: *authPass,
		}
	}
	http.Handle(*metricsEndpoint, handler)
	log.Fatal(http.ListenAndServe(*listeningAddress, nil))
}
