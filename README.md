# nomad prometheus exporter

## Docker

```
  nomon/nomad-exporter:latest
```

## Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| nomad_up | Was the last query of Nomad successful | |
| nomad_raft_peers | How many peers (servers) are in the Raft cluster | |
| nomad_serf_lan_members | How many members are in the cluster | |
| nomad_jobs | How many jobs are in the cluster | |
| nomad_allocations | How many allocations are in the cluster | |
| nomad_allocation_cpu | How much CPU allocation is consuming | job, group, alloc, region, datacenter, node |
| nomad_allocation_cpu_throttle | How much allocation CPU is throttled | job, group, alloc, region, datacenter, node|
| nomad_allocation_memory | How much memory allocation is consuming | job, group, alloc, region, datacenter, node |
| nomad_allocation_memory_limit | Allocation memory limit | job, group, alloc, region, datacenter, node |
| nomad_task_cpu_total_ticks | Task CPU total ticks | job, group, alloc, task, region, datacenter, node |
| nomad_task_cpu_percent | Task CPU usage, percent | job, group, alloc, task, region, datacenter, node |
| nomad_task_memory_rss_bytes | Task memory RSS usage, bytes | job, group, alloc, task, region, datacenter, node |
| nomad_node_resource_memory_megabytes | Amount of allocatable memory the node has in MB | node, datacenter |
| nomad_node_allocated_memory_megabytes | Amount of  memory allocated to tasks on the node in MB | node, datacenter |
| nomad_node_used_memory_megabytes | Amount of memory used on the node in MB | node, datacenter |
| nomad_node_resource_cpu_megahertz | Amount of allocatable CPU the node has in MHz | node, datacenter |
| nomad_node_allocated_cpu_megahertz | Amount of allocated CPU the node has | node, datacenter | 
| nomad_node_used_cpu_megahertz | Amount of CPU used on the node | node, datacenter |

