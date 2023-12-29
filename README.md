# Sentinel Tunnel

[![Release](https://github.com/USA-RedDragon/sentinel_tunnel/actions/workflows/release.yaml/badge.svg)](https://github.com/USA-RedDragon/sentinel_tunnel/actions/workflows/release.yaml) [![go.mod version](https://img.shields.io/github/go-mod/go-version/USA-RedDragon/sentinel_tunnel.svg)](https://github.com/USA-RedDragon/sentinel_tunnel) [![GoReportCard example](https://goreportcard.com/badge/github.com/USA-RedDragon/sentinel_tunnel)](https://goreportcard.com/report/github.com/USA-RedDragon/sentinel_tunnel) [![License](https://badgen.net/github/license/USA-RedDragon/sentinel_tunnel)](https://github.com/USA-RedDragon/sentinel_tunnel/blob/main/LICENSE) [![Release](https://img.shields.io/github/release/USA-RedDragon/sentinel_tunnel.svg)](https://GitHub.com/USA-RedDragon/sentinel_tunnel/releases/) [![Downloads](https://img.shields.io/github/downloads/USA-RedDragon/sentinel_tunnel/total.svg)](https://GitHub.com/USA-RedDragon/sentinel_tunnel/releases/) [![GitHub contributors](https://badgen.net/github/contributors/USA-RedDragon/sentinel_tunnel)](https://GitHub.com/USA-RedDragon/sentinel_tunnel/graphs/contributors/) [![codecov](https://codecov.io/github/USA-RedDragon/sentinel_tunnel/graph/badge.svg?token=YOP6Z4RT3A)](https://codecov.io/github/USA-RedDragon/sentinel_tunnel)

Sentinel Tunnel is a tool that allows you using the Redis Sentinel capabilities, without any code modifications to your application.

Redis Sentinel provides high availability (HA) for Redis. In practical terms this means that using Sentinel you can create a Redis deployment that tolerates certain kinds of failures without human intervention. For more information about Redis Sentinel refer to: <https://redis.io/topics/sentinel>.

## Overview

Connecting an application to a Sentinel-managed Redis deployment is usually done with a Sentinel-aware Redis client. While most Redis clients do support Sentinel, the application needs to call a specialized connection management interface of the client to use it. When one wishes to migrate to a Sentinel-enabled Redis deployment, she/he must modify the application to use Sentinel-based connection management. Moreover, when the application uses a Redis client that does not provide support for Sentinel, the migration becomes that much more complex because it also requires replacing the entire client library.

Sentinel Tunnel (ST) discovers the current Redis master via Sentinel, and creates a TCP tunnel between a local port on the client computer to the master. When the master fails, ST disconnects your client's connection. When the client reconnects, ST rediscovers the current master via Sentinel and provides the new address.
The following diagram illustrates that:

```                                                                                                          _
+----------------------------------------------------------+                                          _,-'*'-,_
| +---------------------------------------+                |                              _,-._      (_ o v # _)
| |                           +--------+  |  +----------+  |    +----------+          _,-'  *  `-._  (_'-,_,-'_)
| |Application code           | Redis  |  |  | Sentinel |  |    |  Redis   | +       (_  O     #  _) (_'|,_,|'_)
| |(uses regular connections) | client +<--->+  Tunnel  +<----->+ Sentinel +<--+---->(_`-._ ^ _,-'_)   '-,_,-'
| |                           +--------+  |  +----------+  |    +----------+ | |     (_`|._`|'_,|'_)
| +---------------------------------------+                |      +----------+ |     (_`|._`|'_,|'_)
| Application node                                         |        +----------+       `-._`|'_,-'
+----------------------------------------------------------+                               `-'
```

## Install

Make sure you have a working Go environment - [see the installation instructions here](http://golang.org/doc/install.html).

To install `sentinel_tunnel`, run:

```bash
go install github.com/USA-RedDragon/sentinel_tunnel@latest
```

Make sure your `PATH` includes the `$GOPATH/bin` directory so your commands can be easily used:

```bash
export PATH=$PATH:$GOPATH/bin
```

## Configure

Configuration can be provided with command line flags, environment variables, or with a config file.

| Env            | Flag          | Description                                 |
|----------------|---------------|---------------------------------------------|
| `ST_CONFIG`    | `--config`    | Config file path                            |
| `ST_SENTINELS` | `--sentinels` | Comma-separated list of Sentinel addresses  |
| `ST_PASSWORD`  | `--password`  | Sentinel password                           |
| `ST_DATABASES` | `--databases` | Comma-separated list of databases to expose |


### Config File

The code contains an example configuration file named [`configuration_example.yaml`](configuration_example.yaml). This file can be modified and used with the `--config` flag or the `ST_CONFIG` env.

## Run

### Manual

In order to run `sentinel_tunnel` manually:

```bash
./sentinel_tunnel \
  --sentinels=redis:6379 \
  --databases=mymaster:6379
```

### Docker

In order to run `sentinel_tunnel` using Docker:

```bash
docker run -d \
  -p 6379:6379 \
  -e ST_SENTINELS=redis:6379
  -e ST_DATABASES=mymaster:6379
  ghcr.io/usa-reddragon/sentinel_tunnel
```

## License

[2-Clause BSD](LICENSE)
