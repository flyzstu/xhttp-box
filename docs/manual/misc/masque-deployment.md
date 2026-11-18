# MASQUE 部署与抗封锁调研

本文整理在 xhttp-box 中部署 `masque-client` / `masque-server` 的 TLS 证书要求、
CDN 与反代兼容性、抗封锁配置要点与已知边界。行为与上游 v1.15.0-alpha.13
一致;xhttp 查询 CDN 场景请改用 XHTTP transport。

## TLS 证书要求

- 默认 `version: 3`(h3/QUIC)下两端都必须启用 TLS:服务端需持有证书,
  客户端只需验证(`tls.enabled` + `server_name`),不上传证书。
- 服务端三种证书来源:现成 `certificate_path`/`key_path`、入站 `acme`
  (HTTP-01 或 DNS 提供商;默认构建 tag 含 `with_acme`)、或自签 +
  客户端 `insecure: true`(仅内网/测试)。
- 仅启用 `version: [1, 2]`(TCP 上的 h1/h2)时技术上可以无 TLS,但等于明文
  代理凭据,不建议。

## CDN 兼容性

| 版本 | 过 CDN | 原因 |
|------|--------|------|
| h3(QUIC datagram) | 否 | CDN 终结 TLS,不透传 QUIC `DATAGRAM` 帧与 `max_datagram_frame_size` |
| h2(extended CONNECT) | 多数不行 | CDN 只对 websocket 类流量放行,通用 `:protocol: connect-ip` 不转发 |
| h1(Upgrade) | 看运气 | 语义上是普通 HTTP/1.1 Upgrade;受 CDN upgrade 白名单、超时与缓冲策略约束 |

客户端回退链(h3 → h2 → h1,`disable_version_fallback` 留空)会自动降级到
CDN 放行的版本,但**能否最终互通取决于 CDN 政策**,不要在 hostile CDN
场景测试 CONNECT-IP;该场景用 XHTTP(纯 HTTP 请求流,CDN 天然放行)。

## 自建 nginx SNI 分流

masque 的 TCP 侧(h1/h2 可与 anytls 相同模式)可挂进 nginx `stream` 的
SNI 直传:

```diff
   map $ssl_preread_server_name $sni_backend {
       {{ anytls_sni }} anytls_backend;
+      {{ masque_sni }} masque_backend;
       {{ xhttp_frontend_domain }} xhttp_https_frontend;
       default          xhttp_https_frontend;
   }

+  upstream masque_backend {
+      server 127.0.0.1:8444;
+  }
+
   upstream anytls_backend {
       server 127.0.0.1:8443;
   }
```

对应 masque server 换一个本地监听端口(如 `8444`);TLS-in-TLS 由后端的
sing-box 终结,SNI 直传不解密。UDP 443(h3)过 `ssl_preread` 的 SNI
分流做不到(仅支持 TCP):要么整端口裸直通(独占端口),要么版本收窄
`"version": [1, 2]`。

## 判定结论

- **直连 443 部署**(首选):三协议同端口 + 真证书 + 认证,流量形态与普通
  HTTPS 站无异,无需额外配置。
- **需要过 hostile CDN**:用 XHTTP;MASQUE 不适用(其定位即"直连前
  端口伪装",不为变形 or 分流设计)。
- **自建 nginx SNI 分流**:TCP 侧完全支持;UDP 侧(h3)要么整端口裸通
  (独占端口),要么版本收窄 `"version": [1, 2]`。
