# XHTTP + AnyTLS SNI fronting

This example shares one public `:443` listener between a normal HTTPS site,
an XHTTP inbound, and an AnyTLS inbound:

```text
XHTTP client / Cloudflare --> :443 stream --> 127.0.0.1:9443 nginx http --> 127.0.0.1:18080 sing-box XHTTP
AnyTLS client             --> :443 stream --> 127.0.0.1:8443 sing-box AnyTLS
```

`stream` only reads TLS SNI. It does not terminate TLS or parse HTTP. The
XHTTP hostname is forwarded to the local HTTP Nginx server, which terminates
TLS, serves the regular site, and proxies only the protected XHTTP path.
AnyTLS is forwarded as raw TCP so that its TLS handshake and post-TLS binary
protocol remain intact.

## Before use

Replace all of the following values in both configuration fragments:

| Placeholder | Use |
| --- | --- |
| `xhttp.example.com` | A real hostname for the normal site and XHTTP. It must be covered by the certificate in `xhttp-site.conf`. |
| `anytls.example.com` | A different real hostname for AnyTLS. It must be covered by the certificate configured in the AnyTLS inbound. |
| `CHANGE_TO_A_LONG_RANDOM_TOKEN` | A randomly generated, stable query token. |
| `/assets/7d91f0c2a8e54b6f/manifest` | A stable XHTTP path. Change it if it collides with site content. |

The XHTTP hostname may be Cloudflare-proxied. The AnyTLS hostname must be
DNS-only when using ordinary Cloudflare proxying: AnyTLS is not HTTP and
cannot pass through Cloudflare's standard HTTP proxy.

For Cloudflare, use **Full (strict)** and an Origin Certificate for the
XHTTP HTTP Nginx server. AnyTLS needs its own TLS certificate in sing-box,
because Nginx passes that TLS connection through without terminating it.

## Nginx placement

* Include `stream-sni.conf` inside Nginx's top-level `stream {}` block. Ensure
  the Nginx stream module is installed.
* Include `xhttp-site.conf` inside the top-level `http {}` block.
* Copy `site/index.html` and `site/404.html` to `/var/www/xhttp-site/`, or
  change the `root` directive accordingly.

The public listener is owned by `stream`. Therefore the HTTP server listens
only on `127.0.0.1:9443`; do not also configure another service to listen on
public port 443.

## sing-box settings

Bind XHTTP to loopback only and use the same hostname and path:

```jsonc
// XHTTP inbound behind Nginx
{
  "listen": "127.0.0.1",
  "listen_port": 18080,
  "transport": {
    "type": "xhttp",
    "host": "xhttp.example.com",
    "path": "/assets/7d91f0c2a8e54b6f/manifest",
    "mode": "packet-up"
  }
}
```

The matching XHTTP client path includes the query token:

```jsonc
{
  "server": "xhttp.example.com",
  "server_port": 443,
  "tls": {
    "enabled": true,
    "server_name": "xhttp.example.com"
  },
  "transport": {
    "type": "xhttp",
    "host": "xhttp.example.com",
    "path": "/assets/7d91f0c2a8e54b6f/manifest?k=CHANGE_TO_A_LONG_RANDOM_TOKEN",
    "mode": "packet-up"
  }
}
```

Nginx validates the query token before proxying. The sing-box XHTTP server
matches the path portion, so it does not need to know the token.

## Operational notes

* Restrict the XHTTP backend port `18080` and AnyTLS backend port `8443` to
  loopback. Only public `443` should be reachable.
* If AnyTLS and Cloudflare-backed XHTTP share an origin IP, the firewall must
  allow Cloudflare on `443` **and** the intended AnyTLS clients. Do not use a
  blanket "Cloudflare IPs only" rule in that topology.
* Test syntax before reload: `nginx -t`, then reload Nginx through your normal
  service manager.
