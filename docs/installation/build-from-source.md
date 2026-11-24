---
icon: material/file-code
---

# Build from source

## :material-graph: Requirements

### sing-box 1.11

* Go 1.23.1 - ~

### sing-box 1.10

* Go 1.20.0 - ~

### sing-box 1.9

* Go 1.18.5 - 1.22.x
* Go 1.20.0 - 1.22.x with tag `with_quic`, or `with_utls` enabled

## :material-fast-forward: Simple Build

```bash
make
```

Or build and install binary to `$GOBIN`:

```bash
make install
```

## :material-cog: Custom Build

```bash
TAGS="tag_a tag_b" make
```

or

```bash
go build -tags "tag_a tag_b" ./cmd/sing-box
```

## :material-folder-settings: Build Tags

| Build Tag                          | Enabled by default   | Description                                                                                                                                                                                                                                                                                                                    |
|------------------------------------|----------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `with_quic`                        | :material-check:     | Build with QUIC support, see [QUIC and HTTP3 DNS transports](/configuration/dns/server/), [Hysteria Inbound](/configuration/inbound/hysteria/), [Hysteria Outbound](/configuration/outbound/hysteria/) and [V2Ray Transport#QUIC](/configuration/shared/v2ray-transport#quic). |
| `with_grpc`                        | :material-close:️    | Build with standard gRPC support, see [V2Ray Transport#gRPC](/configuration/shared/v2ray-transport#grpc).                                                                                                                                                                                                                      |
| `with_dhcp`                        | :material-check:     | Build with DHCP support, see [DHCP DNS transport](/configuration/dns/server/).                                                                                                                                                                                                                                                 |
| `with_wireguard`                   | :material-check:     | Build with WireGuard support, see [WireGuard outbound](/configuration/outbound/wireguard/).                                                                                                                                                                                                                                    |
| `with_utls`                        | :material-check:     | Build with [uTLS](https://github.com/refraction-networking/utls) support for TLS outbound, see [TLS](/configuration/shared/tls#utls).                                                                                                                                                                                          |
| `with_acme`                        | :material-check:     | Build with ACME TLS certificate issuer support, see [TLS](/configuration/shared/tls/).                                                                                                                                                                                                                                         |
| `with_clash_api`                   | :material-check:     | Build with Clash API support, see [Experimental](/configuration/experimental#clash-api-fields).                                                                                                                                                                                                                                |
| `with_v2ray_api`                   | :material-close:️    | Build with V2Ray API support, see [Experimental](/configuration/experimental#v2ray-api-fields).                                                                                                                                                                                                                                |
| `with_gvisor`                      | :material-check:     | Build with gVisor support, see [Tun inbound](/configuration/inbound/tun#stack) and [WireGuard outbound](/configuration/outbound/wireguard#system_interface).                                                                                                                                                                   |
| `with_embedded_tor` (CGO required) | :material-close:️    | Build with embedded Tor support.                                                                                                                                                                                                                                                                                             |
| `with_ccm`                         | :material-check:     | Build with Claude Code Multiplexer service support.                                                                                                                                                                                                                                                                            |
| `with_ocm`                         | :material-check:     | Build with OpenAI Codex Multiplexer service support.                                                                                                                                                                                                                                                                           |
| `with_cloudflared`                 | :material-check:     | Build with Cloudflare Tunnel inbound support, see [Cloudflared inbound](/configuration/inbound/cloudflared/).                                                                                                                                                                                                                  |
| `badlinkname`                      | :material-check:     | Enable `go:linkname` access to internal standard library functions.                                                                                                                                                                                                                                                            |
| `tfogo_checklinkname0`             | :material-check:     | Indicates the build uses the `-checklinkname=0` linker flag. Required together with `badlinkname`.                                                                                                                                                                                                                            |

It is not recommended to change the default build tag list unless you really know what you are adding.

## :material-wrench: Linker Flags

The required linker flags for official builds are maintained in `release/LDFLAGS`. Downstream builds should use that file unchanged.

| Flag                                                        | Description                                                                                                                                                                                                             |
|-------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `-checklinkname=0`                                          | Required together with the `badlinkname` build tag.                                                                                                                                                                     |

## :material-package-variant: For Downstream Packagers

The default build tag lists and linker flags are available as files in the repository for downstream packagers to reference directly:

| File | Description |
|------|-------------|
| `release/DEFAULT_BUILD_TAGS` | Default for Linux (common architectures), Darwin, and Android. |
| `release/DEFAULT_BUILD_TAGS_WINDOWS` | Default for Windows (includes `with_purego`). |
| `release/DEFAULT_BUILD_TAGS_OTHERS` | Default for other platforms. |
| `release/LDFLAGS` | Required linker flags (see above). |

