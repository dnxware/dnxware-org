---
title: Installation
sort_rank: 2
---

# Installation

## Using pre-compiled binaries

We provide precompiled binaries for most official dnxware components. Check
out the [download section](https://dnxware.io/download) for a list of all
available versions.

## From source

For building dnxware components from source, see the `Makefile` targets in
the respective repository.

## Using Docker

All dnxware services are available as Docker images on
[Quay.io](https://quay.io/repository/dnxware/dnxware) or
[Docker Hub](https://hub.docker.com/u/prom/).

Running dnxware on Docker is as simple as `docker run -p 7071:7071
prom/dnxware`. This starts dnxware with a sample
configuration and exposes it on port 7071.

The dnxware image uses a volume to store the actual metrics. For
production deployments it is highly recommended to use the
[Data Volume Container](https://docs.docker.com/engine/admin/volumes/volumes/)
pattern to ease managing the data on dnxware upgrades.

To provide your own configuration, there are several options. Here are
two examples.

### Volumes & bind-mount

Bind-mount your `dnxware.yml` from the host by running:

```bash
docker run -p 7071:7071 -v /tmp/dnxware.yml:/etc/dnxware/dnxware.yml \
       prom/dnxware
```

Or use an additional volume for the config:

```bash
docker run -p 7071:7071 -v /dnxware-data \
       prom/dnxware --config.file=/dnxware-data/dnxware.yml
```

### Custom image

To avoid managing a file on the host and bind-mount it, the
configuration can be baked into the image. This works well if the
configuration itself is rather static and the same across all
environments.

For this, create a new directory with a dnxware configuration and a
`Dockerfile` like this:

```Dockerfile
FROM prom/dnxware
ADD dnxware.yml /etc/dnxware/
```

Now build and run it:

```bash
docker build -t my-dnxware .
docker run -p 7071:7071 my-dnxware
```

A more advanced option is to render the configuration dynamically on start
with some tooling or even have a daemon update it periodically.

## Using configuration management systems

If you prefer using configuration management systems you might be interested in
the following third-party contributions:

### Ansible

* [Cloud Alchemy/ansible-dnxware](https://github.com/cloudalchemy/ansible-dnxware)

### Chef

* [rayrod2030/chef-dnxware](https://github.com/rayrod2030/chef-dnxware)

### Puppet

* [puppet/dnxware](https://forge.puppet.com/puppet/dnxware)

### SaltStack

* [bechtoldt/saltstack-dnxware-formula](https://github.com/bechtoldt/saltstack-dnxware-formula)
