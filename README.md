# xhttp-box

Independent and unofficial XHTTP transport implementation derived from
[SagerNet/sing-box](https://github.com/SagerNet/sing-box).

> [!IMPORTANT]
> This project is independently maintained and is not affiliated with,
> endorsed by, or supported by SagerNet. Please report issues about this
> implementation here rather than to the upstream project.

The `sing-box` name is used only for upstream attribution, source-level
compatibility, and interoperability documentation. Builds and packages
published by this repository use the `xhttp-box` name. See
[NOTICE.md](NOTICE.md) for the complete project identity statement.

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

### Migration from early builds

Early development builds retained the upstream `sing-box` executable and
service names. Current xhttp-box builds use an independent product identity:

- command: `xhttp-box`
- system service: `xhttp-box.service`
- system configuration directory: `/etc/xhttp-box`
- system data directory: `/var/lib/xhttp-box`

The configuration format is unchanged. Copy an existing configuration into
the new directory, update scripts to call `xhttp-box`, and enable the new
service explicitly. No `sing-box` compatibility alias is installed.

## Documentation

- [XHTTP configuration reference](docs/configuration/shared/v2ray-transport.md#xhttp)
- [XHTTP 中文配置参考](docs/configuration/shared/v2ray-transport.zh.md#xhttp)
- [XMUX benchmark guide](docs/manual/misc/xhttp-xmux-benchmark.md)

## Build and test

Build the command-line program:

```bash
go build -o xhttp-box ./cmd/sing-box
```

The source entry point and Go module retain their upstream paths to keep
upstream synchronization practical; the distributed command is `xhttp-box`.

Run the XHTTP test suite:

```bash
go test ./transport/v2rayxhttp
go test -race ./transport/v2rayxhttp
go test -tags with_quic ./transport/v2rayxhttp
go test -tags with_utls ./transport/v2rayxhttp
```

HTTP/3 builds require the `with_quic` build tag.

## Automation

The [upstream synchronization workflow](.github/workflows/upstream-sync.yml)
checks SagerNet/sing-box release tags every day at 04:43 Asia/Shanghai. It
exits when the latest upstream tag is already contained in `dev-next`;
otherwise it attempts a normal Git merge, runs the focused XHTTP tests, and
opens a draft pull request. Copilot CLI is invoked only when the merge has
conflicts or the focused tests fail.

To verify Copilot access without changing the repository, run **Sync Upstream
Releases** manually with **copilot_smoke_test** enabled. The smoke test makes
one minimal Copilot request using the workflow `GITHUB_TOKEN` and fails if the
repository or account policy does not grant `copilot-requests: write`.

Maintainers can publish a version from the current `dev-next` head with the
[versioned release workflow](.github/workflows/release.yml). Run **Publish
Versioned Release**, enter a version such as `0.2.0`, and optionally mark it as
a prerelease. The workflow rejects duplicate or malformed versions, runs the
XHTTP test matrix, builds Linux and Windows archives plus an Alpine package,
generates `SHA256SUMS`, publishes a same-name GitHub Release, and publishes a
versioned GHCR image. Stable releases also update the `latest` container tag.

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

本仓库仅在上游署名、源码级兼容和互操作文档中使用 `sing-box` 名称；本仓库发布的
可执行文件、软件包、服务和镜像均使用 `xhttp-box` 名称。本项目与 SagerNet、
nekohasekai 及 sing-box 原维护团队不存在隶属、赞助或官方认可关系。完整说明见
[NOTICE.md](NOTICE.md)。

早期开发构建沿用了上游的可执行文件和服务名称。当前构建使用 `xhttp-box` 命令、
`xhttp-box.service`、`/etc/xhttp-box` 配置目录和 `/var/lib/xhttp-box` 数据目录。
配置格式本身没有变化；升级时请复制现有配置并更新启动脚本。本项目不会安装
`sing-box` 兼容别名。

## Attribution and license

Copyright and attribution notices from the upstream project are preserved.
Project-specific changes are maintained independently.

Licensed under GPL-3.0 with the additional terms contained in
[LICENSE](LICENSE).
