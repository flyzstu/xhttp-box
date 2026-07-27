# xhttp-box

Independent and unofficial XHTTP transport implementation derived from
[SagerNet/sing-box](https://github.com/SagerNet/sing-box).

> [!IMPORTANT]
> This project is independently maintained and is not affiliated with,
> endorsed by, or supported by SagerNet. Please report issues about this
> implementation here rather than to the upstream project.

[![Release](https://img.shields.io/github/v/release/flyzstu/xhttp-box?display_name=tag)](https://github.com/flyzstu/xhttp-box/releases)
[![XHTTP Lifecycle](https://github.com/flyzstu/xhttp-box/actions/workflows/xhttp-lifecycle.yml/badge.svg?branch=dev-next)](https://github.com/flyzstu/xhttp-box/actions/workflows/xhttp-lifecycle.yml)
[![License](https://img.shields.io/github/license/flyzstu/xhttp-box)](LICENSE)

## Why this project?

xhttp-box adds an Xray-compatible XHTTP transport to the sing-box codebase.
It includes client and server implementations, bidirectional Xray-core
interoperability tests, lifecycle and race testing, and downloadable
development builds.

## Highlights

- `stream-one`, `stream-up`, `packet-up`, and `auto` modes
- HTTP/1.1, HTTP/2, h2c, and optional HTTP/3
- Xray-core client/server interoperability
- REALITY and uTLS interoperability
- XMUX connection reuse and configurable connection/request budgets
- Independent `download_settings` endpoint
- Configurable padding, session, sequence, and uplink-data placement
- Lifecycle, race, HTTP/3, REALITY, XMUX, and protocol test coverage

## Compatibility

| Capability | Status | Notes |
| --- | --- | --- |
| Xray-core client to xhttp-box server | Tested | Covered by interoperability tests |
| xhttp-box client to Xray-core server | Tested | Covered by interoperability tests |
| HTTP/1.1, HTTP/2, and h2c | Supported | Client and server |
| HTTP/3 | Supported | Requires `with_quic`, standard TLS, and ALPN `h3` |
| REALITY and uTLS | Supported | HTTP/3 cannot be combined with REALITY or uTLS |
| XMUX | Supported | Includes concurrency, reuse, request, and lifetime budgets |

## Quick start

Use `xhttp` as the V2Ray transport type in a supported inbound or outbound:

```json
{
  "type": "xhttp",
  "host": "example.com",
  "path": "/xhttp/",
  "mode": "auto",
  "x_padding_bytes": {
    "from": 100,
    "to": 1000
  },
  "sc_max_each_post_bytes": {
    "from": 1000000,
    "to": 1000000
  }
}
```

Client and server settings must be compatible. Start with `mode: "auto"`
unless a specific upload mode or deployment topology requires otherwise.

## Downloads

Versioned builds are published on the
[Releases page](https://github.com/flyzstu/xhttp-box/releases).
The `0.1.0` release includes Linux amd64, Windows amd64, and Alpine x86_64
artifacts.

These builds are experimental. Review the release notes and test them in a
non-critical environment before deployment.

## Documentation

- [XHTTP configuration reference](docs/configuration/shared/v2ray-transport.md#xhttp)
- [XHTTP 中文配置参考](docs/configuration/shared/v2ray-transport.zh.md#xhttp)
- [XMUX benchmark guide](docs/manual/misc/xhttp-xmux-benchmark.md)

## Build and test

Build the command-line program:

```bash
go build ./cmd/sing-box
```

Run the XHTTP test suite:

```bash
go test ./transport/v2rayxhttp
go test -race ./transport/v2rayxhttp
go test -tags with_quic ./transport/v2rayxhttp
go test -tags with_utls ./transport/v2rayxhttp
```

HTTP/3 builds require the `with_quic` build tag.

## Project status and upstream base

XHTTP support is experimental. The current development line started from
upstream `v1.14.0-alpha.45`; subsequent changes are maintained on the
`dev-next` branch. Each release should record its exact upstream base and
project commit so users can evaluate divergence.

The upstream remote and Git history are preserved for attribution and future
synchronization. This repository is a standalone derivative and does not
claim compatibility with every later upstream change.

## 中文说明

xhttp-box 是基于 SagerNet/sing-box 代码的独立、非官方衍生项目，主要增加与
Xray-core 兼容的 XHTTP 客户端和服务端传输层。当前支持三种上传模式、
HTTP/1.1/2/3、REALITY/uTLS、XMUX 和独立下载端点，并提供互操作与生命周期测试。

本项目仍处于实验阶段。配置前请阅读
[中文 XHTTP 文档](docs/configuration/shared/v2ray-transport.zh.md#xhttp)，使用中遇到的
问题请提交到本仓库，不要提交给上游项目。

## Attribution and license

Copyright and attribution notices from the upstream project are preserved.
Project-specific changes are maintained independently.

Licensed under GPL-3.0 with the additional terms contained in
[LICENSE](LICENSE).
