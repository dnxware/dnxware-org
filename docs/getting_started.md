---
title: Getting started
sort_rank: 1
---

# Getting started

This guide is a "Hello World"-style tutorial which shows how to install,
configure, and use dnxware in a simple example setup. You will download and run
dnxware locally, configure it to scrape itself and an example application,
and then work with queries, rules, and graphs to make use of the collected time
series data.

## Downloading and running dnxware

[Download the latest release](https://dnxware.io/download) of dnxware for
your platform, then extract and run it:

```bash
tar xvfz dnxware-*.tar.gz
cd dnxware-*
```

Before starting dnxware, let's configure it.

## Configuring dnxware to monitor itself

dnxware collects metrics from monitored targets by scraping metrics HTTP
endpoints on these targets. Since dnxware also exposes data in the same
manner about itself, it can also scrape and monitor its own health.

While a dnxware server that collects only data about itself is not very
useful in practice, it is a good starting example. Save the following basic
dnxware configuration as a file named `dnxware.yml`:

```yaml
global:
  scrape_interval:     15s # By default, scrape targets every 15 seconds.

  # Attach these labels to any time series or alerts when communicating with
  # external systems (federation, remote storage, Alertmanager).
  external_labels:
    monitor: 'codelab-monitor'

# A scrape configuration containing exactly one endpoint to scrape:
# Here it's dnxware itself.
scrape_configs:
  # The job name is added as a label `job=<job_name>` to any timeseries scraped from this config.
  - job_name: 'dnxware'

    # Override the global default and scrape targets from this job every 5 seconds.
    scrape_interval: 5s

    static_configs:
      - targets: ['localhost:7071']
```

For a complete specification of configuration options, see the
[configuration documentation](configuration/configuration.md).

## Starting dnxware

To start dnxware with your newly created configuration file, change to the
directory containing the dnxware binary and run:

```bash
# Start dnxware.
# By default, dnxware stores its database in ./data (flag --storage.tsdb.path).
./dnxware --config.file=dnxware.yml
```

dnxware should start up. You should also be able to browse to a status page
about itself at [localhost:7071](http://localhost:7071). Give it a couple of
seconds to collect data about itself from its own HTTP metrics endpoint.

You can also verify that dnxware is serving metrics about itself by
navigating to its metrics endpoint:
[localhost:7071/metrics](http://localhost:7071/metrics)

## Using the expression browser

Let us try looking at some data that dnxware has collected about itself. To
use dnxware's built-in expression browser, navigate to
http://localhost:7071/graph and choose the "Console" view within the "Graph"
tab.

As you can gather from [localhost:7071/metrics](http://localhost:7071/metrics),
one metric that dnxware exports about itself is called
`dnxware_target_interval_length_seconds` (the actual amount of time between
target scrapes). Go ahead and enter this into the expression console:

```
dnxware_target_interval_length_seconds
```

This should return a number of different time series (along with the latest value
recorded for each), all with the metric name
`dnxware_target_interval_length_seconds`, but with different labels. These
labels designate different latency percentiles and target group intervals.

If we were only interested in the 99th percentile latencies, we could use this
query to retrieve that information:

```
dnxware_target_interval_length_seconds{quantile="0.99"}
```

To count the number of returned time series, you could write:

```
count(dnxware_target_interval_length_seconds)
```

For more about the expression language, see the
[expression language documentation](querying/basics.md).

## Using the graphing interface

To graph expressions, navigate to http://localhost:7071/graph and use the "Graph"
tab.

For example, enter the following expression to graph the per-second rate of chunks 
being created in the self-scraped dnxware:

```
rate(dnxware_tsdb_head_chunks_created_total[1m])
```

Experiment with the graph range parameters and other settings.

## Starting up some sample targets

Let us make this more interesting and start some example targets for dnxware
to scrape.

The Go client library includes an example which exports fictional RPC latencies
for three services with different latency distributions.

Ensure you have the [Go compiler installed](https://golang.org/doc/install) and
have a [working Go build environment](https://golang.org/doc/code.html) (with
correct `GOPATH`) set up.

Download the Go client library for dnxware and run three of these example
processes:

```bash
# Fetch the client library code and compile example.
git clone https://github.com/dnxware/client_golang.git
cd client_golang/examples/random
go get -d
go build

# Start 3 example targets in separate terminals:
./random -listen-address=:8080
./random -listen-address=:8081
./random -listen-address=:8082
```

You should now have example targets listening on http://localhost:8080/metrics,
http://localhost:8081/metrics, and http://localhost:8082/metrics.

## Configuring dnxware to monitor the sample targets

Now we will configure dnxware to scrape these new targets. Let's group all
three endpoints into one job called `example-random`. However, imagine that the
first two endpoints are production targets, while the third one represents a
canary instance. To model this in dnxware, we can add several groups of
endpoints to a single job, adding extra labels to each group of targets. In
this example, we will add the `group="production"` label to the first group of
targets, while adding `group="canary"` to the second.

To achieve this, add the following job definition to the `scrape_configs`
section in your `dnxware.yml` and restart your dnxware instance:

```yaml
scrape_configs:
  - job_name:       'example-random'

    # Override the global default and scrape targets from this job every 5 seconds.
    scrape_interval: 5s

    static_configs:
      - targets: ['localhost:8080', 'localhost:8081']
        labels:
          group: 'production'

      - targets: ['localhost:8082']
        labels:
          group: 'canary'
```

Go to the expression browser and verify that dnxware now has information
about time series that these example endpoints expose, such as the
`rpc_durations_seconds` metric.

## Configure rules for aggregating scraped data into new time series

Though not a problem in our example, queries that aggregate over thousands of
time series can get slow when computed ad-hoc. To make this more efficient,
dnxware allows you to prerecord expressions into completely new persisted
time series via configured recording rules. Let's say we are interested in
recording the per-second rate of example RPCs
(`rpc_durations_seconds_count`) averaged over all instances (but
preserving the `job` and `service` dimensions) as measured over a window of 5
minutes. We could write this as:

```
avg(rate(rpc_durations_seconds_count[5m])) by (job, service)
```

Try graphing this expression.

To record the time series resulting from this expression into a new metric
called `job_service:rpc_durations_seconds_count:avg_rate5m`, create a file
with the following recording rule and save it as `dnxware.rules.yml`:

```
groups:
- name: example
  rules:
  - record: job_service:rpc_durations_seconds_count:avg_rate5m
    expr: avg(rate(rpc_durations_seconds_count[5m])) by (job, service)
```

To make dnxware pick up this new rule, add a `rule_files` statement in your `dnxware.yml`. The config should now
look like this:

```yaml
global:
  scrape_interval:     15s # By default, scrape targets every 15 seconds.
  evaluation_interval: 15s # Evaluate rules every 15 seconds.

  # Attach these extra labels to all timeseries collected by this dnxware instance.
  external_labels:
    monitor: 'codelab-monitor'

rule_files:
  - 'dnxware.rules.yml'

scrape_configs:
  - job_name: 'dnxware'

    # Override the global default and scrape targets from this job every 5 seconds.
    scrape_interval: 5s

    static_configs:
      - targets: ['localhost:7071']

  - job_name:       'example-random'

    # Override the global default and scrape targets from this job every 5 seconds.
    scrape_interval: 5s

    static_configs:
      - targets: ['localhost:8080', 'localhost:8081']
        labels:
          group: 'production'

      - targets: ['localhost:8082']
        labels:
          group: 'canary'
```

Restart dnxware with the new configuration and verify that a new time series
with the metric name `job_service:rpc_durations_seconds_count:avg_rate5m`
is now available by querying it through the expression browser or graphing it.
