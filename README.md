# nomad prometheus exporter

## Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| nomad_up | Was the last query of Nomad successful | |
| nomad_raft_peers | How many peers (servers) are in the Raft cluster | |
| nomad_serf_lan_members | How many members are in the cluster | |
| nomad_jobs | How many jobs are in the cluster | |
| nomad_allocations | How many allocations are in the cluster | |
| nomad_allocation_cpu | How much CPU allocation is consuming | |
| nomad_allocation_cpu_throttle | How much allocation CPU is throttled | |
| nomad_allocation_memory | How much memory allocation is consuming | |
