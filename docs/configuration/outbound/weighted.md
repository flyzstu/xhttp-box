### Structure

```json
{
  "type": "weighted",
  "tag": "proxy",
  "outbounds": [
    {
      "tag": "proxy-a",
      "weight": 70
    },
    {
      "tag": "proxy-b",
      "weight": 30
    }
  ],
  "strategy": "smooth_wrr",
  "health_check": {
    "enabled": true,
    "url": "https://www.gstatic.com/generate_204",
    "interval": "30s",
    "timeout": "5s",
    "failure_threshold": 3,
    "success_threshold": 2,
    "cooldown": "2m"
  },
  "all_unhealthy": "block"
}
```

The weighted outbound distributes new TCP connections and UDP sessions across
its member outbounds with smooth weighted round-robin. A connection or UDP
session remains on the outbound selected when it was created.

### Fields

#### outbounds

Required. Each member contains an outbound `tag` and a positive relative
`weight`. Weights do not need to add up to 100.

#### strategy

The scheduling strategy. The only supported value and the default is
`smooth_wrr`.

#### health_check

Optional active and passive health checking. When enabled, connection setup
failures and failed URL probes count toward `failure_threshold`. An unhealthy
member is quarantined for `cooldown`, then probed in a half-open state. It
returns to service after `success_threshold` consecutive successful probes.

The defaults are:

- `url`: `https://www.gstatic.com/generate_204`
- `interval`: `30s`
- `timeout`: `5s`
- `failure_threshold`: `3`
- `success_threshold`: `2`
- `cooldown`: `2m`

The failed connection is not retried automatically; subsequent connections
are distributed among the remaining healthy members.

#### all_unhealthy

Behavior when no healthy member supports the requested network:

- `block` (default): fail immediately.
- `fallback`: use the outbound named by `fallback`.

#### fallback

Required when `all_unhealthy` is `fallback`. It must not also appear in
`outbounds`.

### Clash API

The group is exposed as `LoadBalance` through the Clash API. Existing Clash
dashboards can list its members, show the most recently selected member, and
run delay tests. The additional `weighted_status` response field reports each
member's weight and health state; dashboards that do not know this extension
can safely ignore it.
