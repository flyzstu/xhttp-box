### 结构

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

加权负载均衡出站使用平滑加权轮询，将新建 TCP 连接和 UDP 会话分配给成员出站。
连接或 UDP 会话建立后不会中途切换出站。

### 字段

#### outbounds

必填。每个成员包含出站 `tag` 和正整数相对 `weight`，权重之和不必等于 100。

#### strategy

调度策略。目前仅支持且默认使用 `smooth_wrr`。

#### health_check

可选的主动和被动健康检查。启用后，连接建立失败和 URL 探测失败都会计入
`failure_threshold`。失败成员在 `cooldown` 时间内被隔离，随后进入半开探测状态；
连续探测成功达到 `success_threshold` 后恢复分流。

默认值：

- `url`: `https://www.gstatic.com/generate_204`
- `interval`: `30s`
- `timeout`: `5s`
- `failure_threshold`: `3`
- `success_threshold`: `2`
- `cooldown`: `2m`

当前失败的连接不会自动重试；后续连接会分配给剩余健康成员。

#### all_unhealthy

没有健康成员支持所需网络时的行为：

- `block`（默认）：立即失败。
- `fallback`：使用 `fallback` 指定的出站。

#### fallback

当 `all_unhealthy` 为 `fallback` 时必填，且不能同时出现在 `outbounds` 中。

### Clash API

该出站组通过 Clash API 显示为 `LoadBalance`。现有 Clash 面板可以显示成员、最近一次
选择的节点并执行延迟测试。扩展字段 `weighted_status` 提供成员权重与健康状态；不了解
该字段的面板可安全忽略。
