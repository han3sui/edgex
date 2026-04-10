# IoT 平台对接与网关系统指标上报

## 一、功能概述

本次改动实现了两大功能：

1. **IoT 平台北向对接**：新增 `iot_platform` 北向通道类型，网关通过 MQTT 与 IoT 平台通信，接收平台下发的采集配置（通道/设备/测点），并将采集数据上报至平台。
2. **网关自身系统指标采集与上报**：使用 `gopsutil` 采集真实的 CPU、内存、磁盘、网络、WiFi、4G 等系统指标，替换原有的模拟数据，并通过北向通道（IoT 平台 / MQTT）上报。

---

## 二、涉及文件清单

### 2.1 新增文件

| 文件路径 | 说明 |
|---------|------|
| `internal/northbound/iotplatform/client.go` | IoT 平台北向客户端核心逻辑（MQTT 连接、订阅、配置处理、数据上报、系统指标上报） |
| `internal/northbound/iotplatform/models.go` | 平台 MQTT 消息结构体定义（ConfigPush、PropertyPost、GatewayPost、PropertySet、ServiceInvoke 等） |
| `internal/northbound/iotplatform/config_handler.go` | 平台下发配置解析与字段映射转换（协议名映射、连接参数、设备地址、测点参数） |
| `internal/core/sysmon.go` | 系统监控模块，使用 gopsutil 定时采集 CPU/内存/磁盘/网络/WiFi/4G 指标 |
| `ui/src/components/northbound/NorthboundIotPlatform.vue` | IoT 平台北向卡片展示组件 |
| `ui/src/components/northbound/IotPlatformSettingsDialog.vue` | IoT 平台配置表单弹窗（含一键导入 JSON 功能） |
| `conf/gateway_thing_model.json` | 网关自身物模型定义文件（25 个属性、1 个服务、5 个事件） |

### 2.2 修改文件

| 文件路径 | 改动内容 |
|---------|---------|
| `internal/model/types.go` | 新增 `IotPlatformConfig` 结构体（含 `ClientID`、`Username`、`Password`、`ProductID`、`GatewayID` 等字段），在 `NorthboundConfig` 中添加 `IotPlatform` 字段 |
| `internal/core/northbound_manager.go` | 集成 `IotPlatformClient` 生命周期管理、数据分发、系统指标分发（`PublishSystemMetrics`） |
| `internal/server/server.go` | Dashboard 改用 `SysMonitor` 真实数据替换模拟值；新增 `GET /api/system/metrics` 端点；新增 IoT 平台 REST API 路由；`NewServer` 增加 `sysmon` 参数 |
| `internal/server/points_api_test.go` | 适配 `NewServer` 新增参数 |
| `internal/northbound/opcua/server.go` | 修复 `CPUUsage` 节点未更新的 bug，改用 gopsutil 获取真实 CPU/内存/磁盘数据，新增 `DiskUsage` 节点 |
| `internal/northbound/mqtt/client.go` | 新增 `PublishSystemMetrics` 方法，将系统指标发布到 `{base_topic}/$system/metrics` |
| `cmd/main.go` | 创建 `SysMonitor` 实例并启动，传入 `NewServer`，通过 `Subscribe` 将系统指标连接到北向分发 |
| `conf/northbound.yaml` | 添加 `iot_platform` 配置段默认示例（含 `client_id`、`username` 等字段） |
| `ui/src/views/Dashboard.vue` | Dashboard 展示真实系统指标，新增运行时间/网络流量/WiFi/蜂窝信息卡片 |
| `ui/src/views/Northbound.vue` | 集成 IoT 平台类型的配置表单、统计弹窗、添加选项 |
| `ui/src/components/northbound/StatsDialog.vue` | 支持 `iot-platform` 类型的运行监控 |
| `go.mod` / `go.sum` | 新增 `github.com/shirou/gopsutil/v4` 依赖 |

---

## 三、IoT 平台对接

### 3.1 架构

```
IoT 平台 ──MQTT──▶ 网关 (iot_platform client)
                       │
                       ├─ 订阅 /sys/{productID}/{gatewayID}/thing/config/push    → 接收配置下发
                       ├─ 订阅 /sys/{productID}/+/thing/property/set             → 接收属性设置
                       ├─ 订阅 /sys/{productID}/+/thing/service/+/invoke         → 接收服务调用
                       │
                       ├─ 发布 /sys/{productID}/{gatewayID}/thing/property/post  → 网关自身属性上报
                       ├─ 发布 /sys/{productID}/{gatewayID}/thing/gateway/post   → 代理子设备数据上报
                       └─ 发布 /sys/{productID}/{gatewayID}/thing/config/reply   → 配置回复
```

### 3.2 MQTT 连接配置

前端表单支持**一键导入**平台返回的连接 JSON：

```json
{
  "clientId": "25831735_475961002073_20260410110235",
  "username": "25831735:475961002073",
  "passwd": "49c48a5962493e86fcbcd441bf387a93...",
  "mqttHostUrl": "192.168.123.10",
  "port": 1885
}
```

解析后自动填入 Broker（`tcp://host:port`）、Client ID、Username、Password，并从 `username` 中提取 `productID:gatewayID`。

### 3.3 配置下发处理流程

1. 平台发送 `thing.config.push` 消息
2. `config_handler.go` 将平台格式转换为网关内部 `model.Channel` / `model.Device` / `model.Point`
3. 通过 `ChannelManager` 动态添加/替换通道并启动采集
4. 回复配置处理结果

### 3.4 数据上报

| 场景 | Topic | data 格式 |
|------|-------|----------|
| 网关自身属性（系统指标） | `/sys/{productID}/{gatewayID}/thing/property/post` | `{ "cpu_usage": 12.5, "memory_percent": 45.2, ... }` |
| 子设备采集数据 | `/sys/{productID}/{gatewayID}/thing/gateway/post` | `{ "deviceID": { "modelCode": value, ... }, ... }` |

### 3.5 REST API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/northbound/iot-platform` | 创建/更新 IoT 平台配置 |
| DELETE | `/api/northbound/iot-platform/:id` | 删除 IoT 平台配置 |
| GET | `/api/northbound/iot-platform/:id/stats` | 获取运行统计 |

---

## 四、系统指标采集

### 4.1 SysMonitor 模块 (`internal/core/sysmon.go`)

使用 `github.com/shirou/gopsutil/v4`，每 5 秒采集一次：

| 分类 | 指标 | 说明 |
|------|------|------|
| CPU | `cpu_usage` (%) / `cpu_cores` | 真实整机 CPU 使用率 |
| 内存 | `memory_total` / `memory_used` (MB) / `memory_percent` (%) | 整机物理内存 |
| 磁盘 | `disk_total` / `disk_used` / `disk_free` (MB) / `disk_percent` (%) | 根分区 |
| 运行时 | `goroutines` / `go_mem_alloc` (MB) | Go 运行时 |
| 时长 | `uptime` (s) / `system_uptime` (s) | 进程/系统运行时长 |
| 网络 | `net_send_rate` / `net_recv_rate` (KB/s) | 汇总网络速率 |
| WiFi | `wifi_ssid` / `wifi_signal` (dBm) / `wifi_quality` (%) | Linux，通过 iwconfig |
| 蜂窝 | `cell_operator` / `cell_technology` / `cell_rsrp` / `cell_sinr` | Linux，通过 mmcli |

### 4.2 上报通道

| 北向类型 | 上报方式 |
|---------|---------|
| IoT 平台 | `property/post` topic，data 为属性键值对，与物模型 code 对应 |
| MQTT | `{base_topic}/$system/metrics` topic |
| OPC UA | Gateway/Info 地址空间节点（CPUUsage、MemoryUsage、DiskUsage、Goroutines、Uptime） |
| Dashboard API | `GET /api/dashboard/summary` 中的 `system` 字段 |
| 独立 API | `GET /api/system/metrics` 返回完整指标 JSON |

### 4.3 物模型文件

`conf/gateway_thing_model.json` 可直接导入 IoT 平台作为网关产品的物模型：

- **25 个属性**：CPU、内存、磁盘、网络、WiFi、蜂窝等
- **1 个服务**：网关重启（`reboot`）
- **5 个事件**：CPU 过载、内存不足、磁盘空间不足、蜂窝信号弱、网关上线

---

## 五、前端改动

### 5.1 Dashboard (`ui/src/views/Dashboard.vue`)

- 第一行：CPU（含核心数）、内存（含总量/已用）、磁盘（含总量/已用）、运行时间（含协程数/Go 内存）
- 第二行（条件显示）：网络流量（上下行速率 + 网卡列表）、WiFi 信息、蜂窝网络信息

### 5.2 北向配置 (`ui/src/views/Northbound.vue`)

- 添加通道列表中新增「IoT 平台对接」选项
- 集成 `NorthboundIotPlatform` 卡片和 `IotPlatformSettingsDialog` 配置弹窗

### 5.3 IoT 平台配置表单 (`IotPlatformSettingsDialog.vue`)

- **一键导入**：粘贴平台 JSON 自动解析填入
- **MQTT 连接**：Broker / Client ID / Username / Password
- **平台标识**：Product ID / Gateway ID
- **行为配置**：自动启动通道开关

---

## 六、配置示例

`conf/northbound.yaml` 中的 `iot_platform` 配置段：

```yaml
iot_platform:
  - id: iot-platform-1
    name: IoT Platform
    enable: true
    broker: tcp://192.168.123.10:1885
    client_id: "25831735_475961002073_20260410110235"
    username: "25831735:475961002073"
    password: "49c48a5962493e86fcbcd441bf387a93..."
    product_id: "25831735"
    gateway_id: "475961002073"
    auto_start: true
    cache:
      enable: false
      max_count: 0
      flush_interval: ""
```
