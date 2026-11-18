# XHTTP 移植状态

本文记录当前工作区中 XHTTP 的实现边界。目标是以 sing-box 的结构独立重写
Xray XHTTP 的协议行为；没有复制 Xray-core 的源文件或代码片段，新增代码按
sing-box 的 GPL-3.0 项目规则维护。

## 本次已实现

### 集成与配置

- 注册新的 V2Ray transport：`type: "xhttp"`。
- 新增 `V2RayXHTTPOptions`、随机区间和 XMUX 选项的数据模型，并接入 JSON
  编解码。
- 接入现有 VMess、VLESS、Trojan 的通用 V2Ray transport 工厂；三者无需各自
  修改数据面。
- 增加中英文配置文档和当前能力边界说明。

### 协议核心

- 支持 `stream-one`、`stream-up`、`packet-up`；`auto` 当前选择 `packet-up`。
- 支持 path/query/header/cookie 中的 session ID 与 sequence placement。
- 支持 legacy 与 obfs XPadding，含 `repeat-x` 与 `tokenish`。
- 支持 body/header/cookie/auto 上行载荷，header/cookie 使用 URL-safe Base64。
- 服务端支持有界 packet 队列、按 sequence 重组、30 秒未连接 session 回收、
  单请求大小限制和请求头大小限制。
- 使用分离的读写流把 XHTTP 映射成 `net.Conn`，以复用既有入站协议处理。

### HTTP 与性能基础

- 支持 HTTP/1.1、TLS HTTP/2 和 h2c 的 client/server 代码路径。
- HTTP/1.1 stream-one 使用 Go 的 full-duplex 响应控制，避免服务端先耗尽请求
  body 才开始下行响应。
- packet-up 写入使用有界队列、分片、合并、最小发送间隔和共享 HTTP transport；
  H1 transport 保持 idle connection pool，H2 transport 保持可复用的多路连接。
- TLS client 使用 sing-box 的 TLS/dialer 抽象，而非强制标准库 TLS，保留
  uTLS/REALITY 的 H2 接入路径。

## 已验证

- `transport/v2rayxhttp` 的本地端到端回环测试已覆盖三个模式：
  `stream-one`、`stream-up`、`packet-up`。
- 新增可选的本地 Xray 双向互通基线：设置 `XHTTP_XRAY_BINARY` 后，
  `TestXHTTPXRayInterop` 会以自签名 TLS 覆盖 H1/H2、三个 mode，以及
  Xray client / sing-box server 和反方向。运行方式：

  ```bash
  XHTTP_XRAY_BINARY=/path/to/xray go test ./transport/v2rayxhttp -run '^TestXHTTPXRayInterop$' -v
  ```

- 已通过 `go test -race ./transport/v2rayxhttp`。
- 在 Go 1.24.7 下已通过 `go test ./...` 和 `go build ./cmd/sing-box`。
- 当前环境执行 Go 1.24.7 时使用：

  ```bash
  GOSUMDB=sum.golang.org GOTOOLCHAIN=go1.24.7 <go command>
  ```

## 尚未实现或尚未完成验证

### 协议与功能

- HTTP/3 / QUIC 已在 `with_quic` 下实现：TLS ALPN 为 `h3` 时，客户端使用
  独立的 QUIC/HTTP3 transport，服务端监听 UDP；支持 QUIC 的 idle/keep-alive、
  flow-control window、并发流、初始包大小和 PMTU 选项。该路径限定标准 TLS，
  暂不支持 uTLS/REALITY。
- `download_settings` 已实现为嵌入式 XHTTP 客户端配置：可单独指定下行的
  `server`、`server_port`、`tls` 和 XHTTP 字段；上传仍走主目标，二者共享
  session ID。禁止递归配置及与 `stream-one` 组合。
- `auto` 已按 Xray 规则选择：普通 TLS 为 `packet-up`；REALITY 无
  `download_settings` 时为 `stream-one`，有 `download_settings` 时为
  `stream-up`。
- XMUX manager 已实现：按 `max_connections` 或 `max_concurrency` 分配 HTTP
  client，并执行 `c_max_reuse_times`、`h_max_request_times`、
  `h_max_reusable_secs` 预算；packet-up 在请求预算耗尽后自动切换 client。
  H2 `h_keep_alive_period` 已映射为 HTTP/2 空闲探测周期。
- `uplink_chunk_size` 已用于 header/cookie 的动态 Base64 分块，并采用 Xray 的
  默认区间与最小 64 字节约束。
- `sc_stream_up_server_secs` 已实现：当 legacy/obfs padding 生效时，stream-up
  上传响应会发送随机长度的 padding 保活。
- 服务端已写入 Xray 兼容的响应 padding（legacy header、obfs header/cookie）。
- Unix domain socket、浏览器 dialer、UDP hopping、QUIC 拥塞控制等 Xray 特性
  尚未纳入范围。

### 互通、性能与稳定性

- 本地 Xray 26.7.11 双向互通矩阵已通过：H1/H2、`stream-one`、
  `stream-up`、`packet-up`，以及 Xray client / sing-box server 的两个方向
  共 12 项。H1 client 按 Xray 行为禁用 HTTP keep-alive，以避免流式请求
  在连接复用路径上滞留；测试的 Freedom 出站显式允许本机 echo 端点。
- Xray 参数互通矩阵已覆盖双向 obfs `tokenish` padding、session/seq 的
  path/query/header/cookie placement，以及 body/header/cookie/auto 上行载荷。
- VMess、VLESS、Trojan 外层集成矩阵已覆盖 H1/H2、三个 mode 和两个方向。
- 已建立本机 XMUX 对照基准：`BenchmarkXHTTPXMux` 对比 sing-box/Xray 的无
  XMUX 与启用 XMUX 路径，输出吞吐、P99、逻辑连接建立速率、`B/op`、
  `allocs/op`；并支持 Go CPU/内存 profile。运行方法见
  `docs/manual/misc/xhttp-xmux-benchmark.md`。
- 新增 `with_utls` 下的 REALITY/uTLS 双向 Xray 互通测试：H2 的
  `stream-one` 与 `auto` 均覆盖 sing-box client → Xray server 和反方向。
  回落目标为本地自签名 TLS listener，不依赖公网；Xray server 显式设置
  `minClientVer: "0.0.0"`，以允许 sing-box 的兼容 REALITY 版本编码。
  运行方式：

  ```bash
  XHTTP_XRAY_BINARY=/path/to/xray go test -tags with_utls ./transport/v2rayxhttp -run '^TestXHTTPRealityInterop$' -v
  ```

- `download_settings` 已通过本地端到端测试和 Xray 26.7.11 双向互通测试；测试
  使用两个本地入口转发到同一个后端，验证独立下载目标与共享 session。另有
  REALITY/uTLS 的 `auto → stream-up` 双向回归。
- HTTP/3 已通过 `with_quic` 下的本地 H3 回环和 Xray 26.7.11 双向互通矩阵：
  `stream-one`、`stream-up`、`packet-up` 各覆盖两个方向。
- 已补充生命周期压力测试：三个 mode 的单逻辑连接持续 32 次 2 KiB 双向
  传输；packet-up 覆盖乱序到达和缺失 sequence 阻塞、补齐后顺序重组（同时
  覆盖真实 HTTP 请求路径）；孤立 session 使用可缩短的测试超时验证回收和
  stream 关闭；三个 mode 各循环建立 12 条连接，并在 client/server 关闭后验证
  TCP 连接归零和 goroutine 数量保持有界。该套测试也已通过 `-race`。

## 下一步优先级

1. 已新增 `XHTTP Lifecycle` GitHub Actions workflow：相关 XHTTP 提交和 PR 会
   自动运行生命周期 race 测试、默认 transport 测试、`with_quic` 与 `with_utls`
   路径；每周一 02:17 UTC 也会定时运行，支持手动触发。
2. 在具备 `XHTTP_XRAY_BINARY` 的环境继续运行可选的双向互通矩阵。

---

# 其他移植:MASQUE 与统一 HTTP 栈(2026-11)

## MASQUE(RFC 9484 CONNECT-IP)endpoint

- 自上游 v1.15.0-alpha.13 整体移植 `transport/masque/`(session/client/server/
  template/capsule/icmp/pool)、统一 `transport/http/` 栈(tunnel client/server、
  capsule、h3)、`option/masque.go` 与 endpoint 注册,新增
  `masque-client` / `masque-server` 两个 endpoint 类型。
- 外部依赖(sing、sing-tun、quic-go fork)与上游完全同版本,`go.mod` 零改动;
  上游的仓内毛细依赖 `common/badhttp/`、`common/kernelports/` 与
  `adapter.JudgeHostFlow` 随移植补齐。
- 配置文档(`endpoint/masque-{client,server}` 及中文版)与上游一致,
  `docs/schema.json` 已重生成核对。

## http 与 mixed 迁移至统一 transport/http 栈

- `protocol/http/{inbound,outbound}.go` 与 `protocol/mixed/inbound.go` 从
  `sing/protocol/http.HandleConnectionEx` 旧路径切换为仓内 `transport/http`
  server;mixed inbound 以首字节嗅探分流,socks4/5 走原 handshake,HTTP 走
  新栈(认证语义随栈切换:统一栈对 extended CONNECT 强制
  `Proxy-Authorization`,见 `transport/http/tunnel_server.go`)。
- `option/simple.go` 补齐 `HTTPInboundOptions` / `HTTPOutboundOptions` 元
  数据(JSON 变体、schema helper、version 归一化);`route/network.go` 与
  `experimental/libbox` 的 HTTP 前置调用点适配 `transport/http.NewHTTPServer`。

## 新特性验证

- `TestMASQUE*` 7/7 通过(h1/h2/h3 自环、版本回退、通告路由、NoRoute、
  PacketTooBig、重连退避、1.1 升级隧道直测),HTTP/mixed 迁移后既有套件
  全量回归通过。测试需 `with_quic`(h3)与 `with_gvisor`(h2/h3 数据面)。

- 新增真实流量性能测试 `test/transport_perf_test.go`,运行方式:

  ```bash
  go test -tags "with_quic,with_gvisor" -run "TestPerf" -v ./test
  ```

  | 场景 | 结果 |
  |------|------|
  | http(h2 TLS)隧道链 256 MiB echo | 1142 MB/s |
  | masque h3 隧道 256 MiB echo | 209 MB/s |
  | masque h3 隧道 20000×1024B 定速 UDP | 100% 送达,2.1 MB/s |
  | socks→direct 定速 UDP 基线 | 100% 送达,2.1 MB/s |

- 行为备注:UDP 用 2000 pkt/s 定速 + 并发收发;洪泛 burst-then-drain 模式下
  sing 的 socks UDP 中继(纯直连、不过隧道)也会丢失约 43% 报文,为上游
  sing v0.9.7 中继的固有行为,非 masque 移植引入。报文尺寸避开 1200B 以免
  压在 QUIC datagram 容量边界上偶发 `DatagramTooLarge`。
