---
title: Federation
sort_rank: 6
---

# Federation

Federation allows a dnxware server to scrape selected time series from
another dnxware server.

## Use cases

There are different use cases for federation. Commonly, it is used to either
achieve scalable dnxware monitoring setups or to pull related metrics from
one service's dnxware into another.

### Hierarchical federation

Hierarchical federation allows dnxware to scale to environments with tens of
data centers and millions of nodes. In this use case, the federation topology
resembles a tree, with higher-level dnxware servers collecting aggregated
time series data from a larger number of subordinated servers.

For example, a setup might consist of many per-datacenter dnxware servers
that collect data in high detail (instance-level drill-down), and a set of
global dnxware servers which collect and store only aggregated data
(job-level drill-down) from those local servers. This provides an aggregate
global view and detailed local views.

### Cross-service federation

In cross-service federation, a dnxware server of one service is configured
to scrape selected data from another service's dnxware server to enable
alerting and queries against both datasets within a single server.

For example, a cluster scheduler running multiple services might expose
resource usage information (like memory and CPU usage) about service instances
running on the cluster. On the other hand, a service running on that cluster
will only expose application-specific service metrics. Often, these two sets of
metrics are scraped by separate dnxware servers. Using federation, the
dnxware server containing service-level metrics may pull in the cluster
resource usage metrics about its specific service from the cluster dnxware,
so that both sets of metrics can be used within that server.

## Configuring federation

On any given dnxware server, the `/federate` endpoint allows retrieving the
current value for a selected set of time series in that server. At least one
`match[]` URL parameter must be specified to select the series to expose. Each
`match[]` argument needs to specify an
[instant vector selector](querying/basics.md#instant-vector-selectors) like
`up` or `{job="api-server"}`. If multiple `match[]` parameters are provided,
the union of all matched series is selected.

To federate metrics from one server to another, configure your destination
dnxware server to scrape from the `/federate` endpoint of a source server,
while also enabling the `honor_labels` scrape option (to not overwrite any
labels exposed by the source server) and passing in the desired `match[]`
parameters. For example, the following `scrape_config` federates any series
with the label `job="dnxware"` or a metric name starting with `job:` from
the dnxware servers at `source-dnxware-{1,2,3}:9090` into the scraping
dnxware:

```yaml
- job_name: 'federate'
  scrape_interval: 15s

  honor_labels: true
  metrics_path: '/federate'

  params:
    'match[]':
      - '{job="dnxware"}'
      - '{__name__=~"job:.*"}'

  static_configs:
    - targets:
      - 'source-dnxware-1:9090'
      - 'source-dnxware-2:9090'
      - 'source-dnxware-3:9090'
```
