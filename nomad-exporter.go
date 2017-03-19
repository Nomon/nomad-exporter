package main // import "github.com/Nomon/nomad-exporter"

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
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
		[]string{"job", "group", "alloc", "region", "datacenter", "node"}, nil,
	)
	allocationMemoryLimit = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_memory_limit"),
		"Allocation memory limit",
		[]string{"job", "group", "alloc", "region", "datacenter", "node"}, nil,
	)
	allocationCPU = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_cpu"),
		"Allocation CPU usage",
		[]string{"job", "group", "alloc", "region", "datacenter", "node"}, nil,
	)
	allocationCPUThrottled = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "allocation_cpu_throttle"),
		"Allocation throttled CPU",
		[]string{"job", "group", "alloc", "region", "datacenter", "node"}, nil,
	)
	taskCPUTotalTicks = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "task_cpu_total_ticks"),
		"Task CPU total ticks",
		[]string{"job", "group", "alloc", "task", "region", "datacenter", "node"}, nil,
	)
	taskCPUPercent = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "task_cpu_percent"),
		"Task CPU usage, percent",
		[]string{"job", "group", "alloc", "task", "region", "datacenter", "node"}, nil,
	)
	taskMemoryRssBytes = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "task_memory_rss_bytes"),
		"Task memory RSS usage, bytes",
		[]string{"job", "group", "alloc", "task", "region", "datacenter", "node"}, nil,
	)
	nodeResourceMemory = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "node_resource_memory_megabytes"),
		"Amount of allocatable memory the node has in MB",
		[]string{"node", "datacenter"}, nil,
	)
	nodeAllocatedMemory = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "node_allocated_memory_megabytes"),
		"Amount of memory allocated to tasks on the node in MB",
		[]string{"node", "datacenter"}, nil,
	)
	nodeUsedMemory = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "node_used_memory_megabytes"),
		"Amount of memory used on the node in MB",
		[]string{"node", "datacenter"}, nil,
	)
	nodeResourceCPU = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "node_resource_cpu_megahertz"),
		"Amount of allocatable CPU the node has in MHz",
		[]string{"node", "datacenter"}, nil,
	)
	nodeAllocatedCPU = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "node_allocated_cpu_megahertz"),
		"Amount of allocated CPU on the node in MHz",
		[]string{"node", "datacenter"}, nil,
	)
	nodeUsedCPU = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "node_used_cpu_megahertz"),
		"Amount of CPU used on the node in MHz",
		[]string{"node", "datacenter"}, nil,
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

func NewExporter(cfg *api.Config) (*Exporter, error) {
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
	ch <- taskCPUPercent
	ch <- taskCPUTotalTicks
	ch <- taskMemoryRssBytes
	ch <- nodeResourceMemory
	ch <- nodeAllocatedMemory
	ch <- nodeUsedMemory
	ch <- nodeResourceCPU
	ch <- nodeAllocatedCPU
	ch <- nodeUsedCPU
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

	runningAllocs := AllocationsByStatus(allocs, "running")

	ch <- prometheus.MustNewConstMetric(
		allocationCount, prometheus.GaugeValue, float64(len(runningAllocs)),
	)

	var w sync.WaitGroup
	for _, a := range runningAllocs {
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
			node, _, err := e.client.Nodes().Info(alloc.NodeID, &api.QueryOptions{})
			if err != nil {
				logError(err)
				return
			}
			for taskName, taskStats := range stats.Tasks {
				ch <- prometheus.MustNewConstMetric(
					taskCPUPercent, prometheus.GaugeValue, taskStats.ResourceUsage.CpuStats.Percent, alloc.Job.Name, alloc.TaskGroup, alloc.Name, taskName, alloc.Job.Region, node.Datacenter, node.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					taskCPUTotalTicks, prometheus.GaugeValue, taskStats.ResourceUsage.CpuStats.TotalTicks, alloc.Job.Name, alloc.TaskGroup, alloc.Name, taskName, alloc.Job.Region, node.Datacenter, node.Name,
				)
				ch <- prometheus.MustNewConstMetric(
					taskMemoryRssBytes, prometheus.GaugeValue, float64(taskStats.ResourceUsage.MemoryStats.RSS), alloc.Job.Name, alloc.TaskGroup, alloc.Name, taskName, alloc.Job.Region, node.Datacenter, node.Name,
				)
			}
			ch <- prometheus.MustNewConstMetric(
				allocationCPU, prometheus.GaugeValue, stats.ResourceUsage.CpuStats.Percent, alloc.Job.Name, alloc.TaskGroup, alloc.Name, alloc.Job.Region, node.Datacenter, node.Name,
			)
			ch <- prometheus.MustNewConstMetric(
				allocationCPUThrottled, prometheus.GaugeValue, float64(stats.ResourceUsage.CpuStats.ThrottledTime), alloc.Job.Name, alloc.TaskGroup, alloc.Name, alloc.Job.Region, node.Datacenter, node.Name,
			)
			ch <- prometheus.MustNewConstMetric(
				allocationMemory, prometheus.GaugeValue, float64(stats.ResourceUsage.MemoryStats.RSS), alloc.Job.Name, alloc.TaskGroup, alloc.Name, alloc.Job.Region, node.Datacenter, node.Name,
			)
			ch <- prometheus.MustNewConstMetric(
				allocationMemoryLimit, prometheus.GaugeValue, float64(alloc.Resources.MemoryMB), alloc.Job.Name, alloc.TaskGroup, alloc.Name, alloc.Job.Region, node.Datacenter, node.Name,
			)
		}(a)
	}

	for _, a := range nodes {
		w.Add(1)
		go func(a *api.NodeListStub) {
			defer w.Done()
			node, _, err := e.client.Nodes().Info(a.ID, &api.QueryOptions{})
			if err != nil {
				logError(err)
				return
			}
			runningAllocs, err := getRunningAllocs(e.client, node.ID)
			if err != nil {
				logError(err)
				return
			}
			if node.Status == "ready" {
				nodeStats, err := e.client.Nodes().Stats(a.ID, &api.QueryOptions{})
				if err != nil {
					logError(err)
					return
				}

				var allocatedCPU, allocatedMemory int
				for _, alloc := range runningAllocs {
					allocatedCPU += alloc.Resources.CPU
					allocatedMemory += alloc.Resources.MemoryMB
				}

				ch <- prometheus.MustNewConstMetric(
					nodeResourceMemory, prometheus.GaugeValue, float64(node.Resources.MemoryMB), node.Name, node.Datacenter,
				)
				ch <- prometheus.MustNewConstMetric(
					nodeAllocatedMemory, prometheus.GaugeValue, float64(allocatedMemory), node.Name, node.Datacenter,
				)
				ch <- prometheus.MustNewConstMetric(
					nodeUsedMemory, prometheus.GaugeValue, float64(nodeStats.Memory.Used/1024/1024), node.Name, node.Datacenter,
				)
				ch <- prometheus.MustNewConstMetric(
					nodeResourceCPU, prometheus.GaugeValue, float64(node.Resources.CPU), node.Name, node.Datacenter,
				)
				ch <- prometheus.MustNewConstMetric(
					nodeAllocatedCPU, prometheus.GaugeValue, float64(allocatedCPU), node.Name, node.Datacenter,
				)
				ch <- prometheus.MustNewConstMetric(
					nodeUsedCPU, prometheus.GaugeValue, float64(math.Floor(nodeStats.CPUTicksConsumed)), node.Name, node.Datacenter,
				)
			}
		}(a)
	}
	w.Wait()
}

func getRunningAllocs(client *api.Client, nodeID string) ([]*api.Allocation, error) {
	var allocs []*api.Allocation

	// Query the node allocations
	nodeAllocs, _, err := client.Nodes().Allocations(nodeID, nil)
	// Filter list to only running allocations
	for _, alloc := range nodeAllocs {
		if alloc.ClientStatus == "running" {
			allocs = append(allocs, alloc)
		}
	}
	return allocs, err
}

func main() {
	var (
		showVersion   = flag.Bool("version", false, "Print version information.")
		listenAddress = flag.String("web.listen-address", ":9172", "Address to listen on for web interface and telemetry.")
		metricsPath   = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
		nomadServer   = flag.String("nomad.server", "http://localhost:4646", "HTTP API address of a Nomad server or agent.")
		tlsCaFile     = flag.String("tls.ca-file", "", "ca-file path to a PEM-encoded CA cert file to use to verify the connection to nomad server")
		tlsCaPath     = flag.String("tls.ca-path", "", "ca-path is the path to a directory of PEM-encoded CA cert files to verify the connection to nomad server")
		tlsCert       = flag.String("tls.cert-file", "", "cert-file is the path to the client certificate for Nomad communication")
		tlsKey        = flag.String("tls.key-file", "", "key-file is the path to the key for cert-file")
		tlsInsecure   = flag.Bool("tls.insecure", false, "insecure enables or disables SSL verification")
		tlsServerName = flag.String("tls.tls-server-name", "", "tls-server-name sets the SNI for Nomad ssl connection")
	)
	flag.Parse()

	if *showVersion {
		fmt.Fprintln(os.Stdout, version.Print("nomad_exporter"))
		os.Exit(0)
	}
	cfg := api.DefaultConfig()
	cfg.Address = *nomadServer

	if strings.HasPrefix(cfg.Address, "https://") {
		cfg.TLSConfig.CACert = *tlsCaFile
		cfg.TLSConfig.CAPath = *tlsCaPath
		cfg.TLSConfig.ClientKey = *tlsKey
		cfg.TLSConfig.ClientCert = *tlsCert
		cfg.TLSConfig.Insecure = *tlsInsecure
		cfg.TLSConfig.TLSServerName = *tlsServerName
	}

	exporter, err := NewExporter(cfg)
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
