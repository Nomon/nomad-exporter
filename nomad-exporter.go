package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/hashicorp/nomad/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
)

const (
	namespace = "nomad"
)

var (
	up = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "up"),
		"Was the last query of Nomad successful.",
		nil, nil,
	)
	clusterServers = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "raft_peers"),
		"How many peers (servers) are in the Raft cluster.",
		nil, nil,
	)
	nodeCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "serf_lan_members"),
		"How many members are in the cluster.",
		nil, nil,
	)
	jobCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "serf_lan_members"),
		"How many jobs are there in the cluster.",
		nil, nil,
	)
	allocationCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "serf_lan_members"),
		"How many allocations are there in the cluster.",
		nil, nil,
	)
	nodes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "nodes"),
		"How many jobs are running in the cluster.",
		[]string{"job", "group", "status"}, nil,
	)

	jobs = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocations"),
		"How many jobs are running in the cluster.",
		[]string{"job", "group", "status"}, nil,
	)
	allocation = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocations"),
		"Allocations",
		[]string{"job", "group", "status", "node", "name", "region", "dc"}, nil,
	)
	allocationMemory = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_memory"),
		"Allocations",
		[]string{"alloc", "group", "job", "pid"}, nil,
	)
	allocationCPU = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_cpu"),
		"CPU",
		[]string{"alloc", "group", "job", "pid", "mode"}, nil,
	)
	allocationCPUThrottled = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_cpu"),
		"Throttled CPU",
		[]string{"alloc", "group", "job", "pid"}, nil,
	)
)

type Exporter struct {
	client *api.Client
}

func NewExporter(address string) (*Exporter, error) {
	cfg := api.DefaultConfig()
	cfg.Address = address
	client, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &Exporter{
		client: client,
	}, nil
}

// Describe implements Collector interface.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- up
	ch <- clusterServers
	ch <- nodeCount
	ch <- allocationCount
	ch <- jobCount
	ch <- nodes
	ch <- jobs
	ch <- allocation
	ch <- allocationMemory
	ch <- allocationCPU
	ch <- allocationCPUThrottled
}

// Collect collects nomad metrics
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	peers, err := e.client.Status().Peers()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,
		)
		log.Println("Query failed", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		up, prometheus.GaugeValue, 1,
	)
	ch <- prometheus.MustNewConstMetric(
		clusterServers, prometheus.GaugeValue, float64(len(peers)),
	)
	nodes, _, err := e.client.Nodes().List(&api.QueryOptions{})
	if err != nil {
		log.Println("Query failed", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		nodeCount, prometheus.GaugeValue, float64(len(nodes)),
	)
	jobs, _, err := e.client.Jobs().List(&api.QueryOptions{})
	if err != nil {
		log.Println("Query failed", err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		jobCount, prometheus.GaugeValue, float64(len(jobs)),
	)

}

func main() {
	var (
		showVersion   = flag.Bool("version", false, "Print version information.")
		listenAddress = flag.String("web.listen-address", ":9172", "Address to listen on for web interface and telemetry.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		nomadServer   = flag.String("nomad.server", "localhost:4646", "HTTP API address of a Nomad server or agent.")
	)
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("nomad_exporter"))
		os.Exit(0)
	}
	exporter, err := NewExporter(*nomadServer)
	if err != nil {
		log.Fatal(err)
	}
	prometheus.MustRegister(exporter)

	http.Handle(*metricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>Nomad Exporter</title></head>
             <body>
             <h1>Nomad Exporter</h1>
             <p><a href='` + *metricsPath + `'>Metrics</a></p>
             </body>
             </html>`))
	})

	log.Println("Listening on", *listenAddress)
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
