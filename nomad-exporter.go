package main // import "github.com/Nomon/nomad-exporter"

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

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
		prometheus.BuildFQName(namespace, "", "jobs"),
		"How many jobs are there in the cluster.",
		nil, nil,
	)
	allocationCount = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocations"),
		"How many allocations are there in the cluster.",
		nil, nil,
	)
	allocationMemory = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_memory"),
		"Allocation memory usage",
		[]string{"job", "group", "alloc"}, nil,
	)
	allocationMemoryLimit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_memory_limit"),
		"Allocation memory limit",
		[]string{"job", "group", "alloc"}, nil,
	)
	allocationCPU = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_cpu"),
		"Allocation CPU usage",
		[]string{"job", "group", "alloc"}, nil,
	)
	allocationCPUThrottled = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_cpu_throttle"),
		"Allocation throttled CPU",
		[]string{"job", "group", "alloc"}, nil,
	)
)

func AllocationsByStatus(allocs []*api.AllocationListStub, status string) []*api.AllocationListStub {
	var resp []*api.AllocationListStub
	for _, a := range allocs {
		if a.ClientStatus == status {
			resp = append(resp, a)
		}
	}
	return resp
}

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
	ch <- allocationMemory
	ch <- allocationCPU
	ch <- allocationCPUThrottled
	ch <- allocationMemoryLimit
}

// Collect collects nomad metrics
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	peers, err := e.client.Status().Peers()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(
			up, prometheus.GaugeValue, 0,
		)
		logError(err)
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
		logError(err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		nodeCount, prometheus.GaugeValue, float64(len(nodes)),
	)
	jobs, _, err := e.client.Jobs().List(&api.QueryOptions{})
	if err != nil {
		logError(err)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		jobCount, prometheus.GaugeValue, float64(len(jobs)),
	)
	allocs, _, err := e.client.Allocations().List(&api.QueryOptions{})
	if err != nil {
		logError(err)
		return
	}

	running_allocs := AllocationsByStatus(allocs, "running")

	ch <- prometheus.MustNewConstMetric(
		allocationCount, prometheus.GaugeValue, float64(len(running_allocs)),
	)

	var w sync.WaitGroup
	for _, a := range running_allocs {
		w.Add(1)
		go func(a *api.AllocationListStub) {
			defer w.Done()
			alloc, _, err := e.client.Allocations().Info(a.ID, &api.QueryOptions{})
			if err != nil {
				logError(err)
				return
			}

			stats, err := e.client.Allocations().Stats(alloc, &api.QueryOptions{})
			if err != nil {
				logError(err)
				return
			}
			ch <- prometheus.MustNewConstMetric(
				allocationCPU, prometheus.GaugeValue, stats.ResourceUsage.CpuStats.Percent, alloc.Job.Name, alloc.TaskGroup, alloc.Name,
			)
			ch <- prometheus.MustNewConstMetric(
				allocationCPUThrottled, prometheus.GaugeValue, float64(stats.ResourceUsage.CpuStats.ThrottledTime), alloc.Job.Name, alloc.TaskGroup, alloc.Name,
			)
			ch <- prometheus.MustNewConstMetric(
				allocationMemory, prometheus.GaugeValue, float64(stats.ResourceUsage.MemoryStats.RSS), alloc.Job.Name, alloc.TaskGroup, alloc.Name,
			)
			ch <- prometheus.MustNewConstMetric(
				allocationMemoryLimit, prometheus.GaugeValue, float64(alloc.Resources.MemoryMB), alloc.Job.Name, alloc.TaskGroup, alloc.Name,
			)
		}(a)
	}
	w.Wait()
}

func main() {
	var (
		showVersion   = flag.Bool("version", false, "Print version information.")
		listenAddress = flag.String("web.listen-address", ":9172", "Address to listen on for web interface and telemetry.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		nomadServer   = flag.String("nomad.server", "http://localhost:4646", "HTTP API address of a Nomad server or agent.")
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

func logError(err error) {
	log.Println("Query error", err)
	return
}
