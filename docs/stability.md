---
title: API Stability
sort_rank: 8
---

# API Stability Guarantees

dnxware promises API stability within a major version, and strives to avoid
breaking changes for key features. Some features, which are cosmetic, still
under development, or depend on 3rd party services, are not covered by this.

Things considered stable for 2.x:

* The query language and data model
* Alerting and recording rules
* The ingestion exposition format
* v1 HTTP API (used by dashboards and UIs)
* Configuration file format (minus the service discovery remote read/write, see below)
* Rule/alert file format
* Console template syntax and semantics

Things considered unstable for 2.x:

* Any feature listed as experimental or subject to change, including:
  * The [`holt_winters` PromQL function](https://github.com/dnxware/dnxware/issues/2458)
  * Remote read, remote write and the remote read endpoint
  * v2 HTTP and GRPC APIs
* Service discovery integrations, with the exception of `static_configs` and `file_sd_configs`
* Go APIs of packages that are part of the server
* HTML generated by the web UI
* The metrics in the /metrics endpoint of dnxware itself
* Exact on-disk format. Potential changes however, will be forward compatible and transparently handled by dnxware

As long as you are not using any features marked as experimental/unstable, an
upgrade within a major version can usually be performed without any operational
adjustments and very little risk that anything will break. Any breaking changes
will be marked as `CHANGE` in release notes.

