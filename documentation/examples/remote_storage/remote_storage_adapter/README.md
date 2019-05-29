# Remote storage adapter

This is a write adapter that receives samples via dnxware's remote write
protocol and stores them in Graphite, InfluxDB, or OpenTSDB. It is meant as a
replacement for the built-in specific remote storage implementations that have
been removed from dnxware.

For InfluxDB, this binary is also a read adapter that supports reading back
data through dnxware via dnxware's remote read protocol.

## Building

```
go build
```

## Running

Graphite example:

```
./remote_storage_adapter -graphite-address=localhost:8080
```

OpenTSDB example:

```
./remote_storage_adapter -opentsdb-url=http://localhost:8081/
```

InfluxDB example:

```
./remote_storage_adapter -influxdb-url=http://localhost:8086/ -influxdb.database=dnxware -influxdb.retention-policy=autogen
```

To show all flags:

```
./remote_storage_adapter -h
```

## Configuring dnxware

To configure dnxware to send samples to this binary, add the following to your `dnxware.yml`:

```yaml
# Remote write configuration (for Graphite, OpenTSDB, or InfluxDB).
remote_write:
  - url: "http://localhost:9201/write"

# Remote read configuration (for InfluxDB only at the moment).
remote_read:
  - url: "http://localhost:9201/read"
```
