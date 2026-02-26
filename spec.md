# Modbus 驱动深度优化技术规格书

## 1. 核心架构升级
在保持 `simonvetter/modbus` 底层库不变的前提下，通过上层封装实现智能调度与自适应机制。

### 1.1 模块划分
- **TransportLayer**: 负责 TCP 连接管理、长连接维护、心跳保活、自动重连（基于 simonvetter 库封装）。
- **SmartScheduler**: 负责自适应调度、批量优化、地址合并、流量控制。
- **ProtocolEngine**: 负责数据编解码、零拷贝解析、类型转换。
- **ReliabilityManager**: 负责熔断保护、故障恢复、智能探测。
- **MetricsCollector**: 负责全维度指标监控与统计。

## 2. 详细设计方案

### 2.1 连接管理 (Connection Management)
- **单一长连接**: 
  - 每个采集通道（Channel）仅维护一个 `modbus.Client` 实例。
  - 所有下属设备（Device）共享此连接，通过 `SetSlaveID` 切换上下文。
  - 禁用 `simonvetter` 库内部的自动重连（如果可能），或在上层实现更精细的退避策略。
- **指数退避重连**:
  - 在 `Transport` 层封装 `Connect` 方法。
  - 失败时执行退避：`wait = min(max_wait, base * 2^retry_count)`。
  - 初始间隔 1s，最大 300s。


### 2.2 智能探测 (Intelligent Probing)
- **MTU 探测**:
  - 连接建立后，二分法尝试读取 [32, 125] 个寄存器。
  - 确定当前链路的最大 PDU 长度，设置 `MaxBatchSize`。
- **有效区间探测**:
  - 基于配置的点位地址，向前后尝试读取，构建 `ValidAddressMap`。
  - 避免无效地址导致的读取失败。
- **响应时间建模**:
  - 记录不同 `BatchSize` 下的 RTT。
  - 用于调度器决策最佳批量大小。

### 2.3 批量读取优化 (Batch Optimization)
- **AIMD 算法**:
  - 动态调整 `BatchSize`。
  - 成功率 > 95% 且延迟低 -> `BatchSize += 8`。
  - 失败或超时 -> `BatchSize *= 0.75`。
- **动态合并**:
  - 对排序后的点位进行分组。
  - `Gap <= DynamicThreshold` 时合并请求。

### 2.4 数据解析 (Zero-Copy Decoding)
- **零拷贝实现**:
  - 直接操作 `[]byte`，使用 `binary.BigEndian`。
  - 避免 `bytes.Reader` 和切片拷贝。
- **全类型支持**:
  - 优化所有 16 种数据格式的解析路径。

### 2.5 自适应调度 (Adaptive Scheduling)
- **动态采集间隔**:
  - 目标负载率 30%-50%。
  - `NextInterval` 根据 `LastDuration` 动态计算。
- **背压控制**:
  - 数据处理队列积压时，主动延长采集间隔。

### 2.6 熔断保护 (Circuit Breaker)
- **点位级**: 连续 3 次失败 -> 熔断 5 分钟。
- **设备级**: 失败率 > 50% -> 降级模式。

### 2.7 监控指标 (Metrics)
- **Counter**: `tx_total`, `rx_total`, `errors_total`.
- **Gauge**: `connected`, `latency`, `batch_size`.
- **Histogram**: `response_time`.

## 3. 接口变更
- `ModbusDriver` 内部结构调整，引入 `Transport` 和 `Scheduler` 接口。
- 配置文件 `DriverConfig` 增加优化参数（默认关闭或自动模式）。

## 4. 兼容性说明
- 完全兼容现有 `channels.yaml`。
- 如果需要可以增加 Modbus-TCP 配置项目(那么UI也需要调整)
- 对外 API 保持不变。
