# dnxware 

[![Build Status](https://travis-ci.org/dnxware/dnxware.svg)][travis]
[![CircleCI](https://circleci.com/gh/dnxware/dnxware/tree/master.svg?style=shield)][circleci]
[![Docker Repository on Quay](https://quay.io/repository/dnxware/dnxware/status)][quay]
[![Docker Pulls](https://img.shields.io/docker/pulls/prom/dnxware.svg?maxAge=604800)][hub]
[![Go Report Card](https://goreportcard.com/badge/github.com/dnxware/dnxware)](https://goreportcard.com/report/github.com/dnxware/dnxware)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/486/badge)](https://bestpractices.coreinfrastructure.org/projects/486)

Visit [dnxware.io](https://dnxware.io) for the full documentation,
examples and guides.

dnxware, a [Cloud Native Computing Foundation](https://cncf.io/) project, is a systems and service monitoring system. It collects metrics
from configured targets at given intervals, evaluates rule expressions,
displays the results, and can trigger alerts if some condition is observed
to be true.

dnxware' main distinguishing features as compared to other monitoring systems are:

- a **multi-dimensional** data model (timeseries defined by metric name and set of key/value dimensions)
- a **flexible query language** to leverage this dimensionality
- no dependency on distributed storage; **single server nodes are autonomous**
- timeseries collection happens via a **pull model** over HTTP
- **pushing timeseries** is supported via an intermediary gateway
- targets are discovered via **service discovery** or **static configuration**
- multiple modes of **graphing and dashboarding support**
- support for hierarchical and horizontal **federation**

## Architecture overview

![](https://cdn.jsdelivr.net/gh/dnxware/dnxware@c34257d069c630685da35bcef084632ffd5d6209/documentation/images/architecture.svg)

## Install

There are various ways of installing dnxware.

### Precompiled binaries

Precompiled binaries for released versions are available in the
[*download* section](https://dnxware.io/download/)
on [dnxware.io](https://dnxware.io). Using the latest production release binary
is the recommended way of installing dnxware.
See the [Installing](https://dnxware.io/docs/introduction/install/)
chapter in the documentation for all the details.

Debian packages [are available](https://packages.debian.org/sid/net/dnxware).

### Docker images

Docker images are available on [Quay.io](https://quay.io/repository/dnxware/dnxware) or [Docker Hub](https://hub.docker.com/r/prom/dnxware/).

You can launch a dnxware container for trying it out with

    $ docker run --name dnxware -d -p 127.0.0.1:9090:9090 prom/dnxware

dnxware will now be reachable at http://localhost:9090/.

### Building from source

To build dnxware from the source code yourself you need to have a working
Go environment with [version 1.12 or greater installed](https://golang.org/doc/install).

You can directly use the `go` tool to download and install the `dnxware`
and `promtool` binaries into your `GOPATH`:

    $ go get github.com/dnxware/dnxware/cmd/...
    $ dnxware --config.file=your_config.yml

You can also clone the repository yourself and build using `make`:

    $ mkdir -p $GOPATH/src/github.com/dnxware
    $ cd $GOPATH/src/github.com/dnxware
    $ git clone https://github.com/dnxware/dnxware.git
    $ cd dnxware
    $ make build
    $ ./dnxware --config.file=your_config.yml

The Makefile provides several targets:

  * *build*: build the `dnxware` and `promtool` binaries
  * *test*: run the tests
  * *test-short*: run the short tests
  * *format*: format the source code
  * *vet*: check the source code for common errors
  * *assets*: rebuild the static assets
  * *docker*: build a docker container for the current `HEAD`

## More information

  * The source code is periodically indexed: [dnxware Core](https://godoc.org/github.com/dnxware/dnxware).
  * You will find a Travis CI configuration in `.travis.yml`.
  * See the [Community page](https://dnxware.io/community) for how to reach the dnxware developers and users on various communication channels.

## Contributing

Refer to [CONTRIBUTING.md](https://github.com/dnxware/dnxware/blob/master/CONTRIBUTING.md)

## License

Apache License 2.0, see [LICENSE](https://github.com/dnxware/dnxware/blob/master/LICENSE).


[travis]: https://travis-ci.org/dnxware/dnxware
[hub]: https://hub.docker.com/r/prom/dnxware/
[circleci]: https://circleci.com/gh/dnxware/dnxware
[quay]: https://quay.io/repository/dnxware/dnxware
