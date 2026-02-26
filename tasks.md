# Modbus 驱动深度优化任务清单

## 1. 核心连接管理 (Transport Layer)
- [ ] **TCP 连接池优化**: 确保 `ModbusTransport` 只持有一个 `modbus.Client`，并通过 `SetSlaveID` 复用连接。 <!-- id: tcp-reuse -->
- [ ] **指数退避重连**: 在 `Connect` 方法外围实现重试循环，使用 1s, 2s, 4s... 300s 的退避策略。 <!-- id: backoff-retry -->
- [ ] **心跳保活**: 实现独立 Goroutine，每 30s 执行一次轻量级读取（如读取 当前设备点位第一个地址），失败 3 次主动断开。 <!-- id: heartbeat -->

## 2. 智能探测与优化 (Intelligent Probing)
- [ ] **MTU 探测**: 连接建立后，尝试读取 32-125 个寄存器，确定 `MaxBatchSize`。 <!-- id: mtu-probe -->
- [ ] **响应时间记录**: 维护 `map[batchSize]duration`，记录不同批量大小下的平均 RTT。 <!-- id: rtt-stats -->
- [ ] **有效地址学习**: 记录 `IllegalDataAddress` 错误，构建无效地址黑名单，避免无效读取。 <!-- id: address-learning -->

## 3. 调度器增强 (Scheduler Enhancement)
- [ ] **AIMD 算法**: 实现加性增乘性减算法动态调整 `BatchSize`。 <!-- id: aimd-algo -->
- [ ] **动态合并**: 根据 `BatchSize` 和网络状况动态调整合并间隙 `MaxGap`。 <!-- id: dynamic-merge -->
- [ ] **自适应间隔**: 采集循环中计算 `LastDuration`，动态调整 `Ticker`。 <!-- id: adaptive-interval -->
- [ ] **背压控制**: 监控 `DataBuffer` 通道长度，积压时暂停或减缓采集。 <!-- id: backpressure -->

## 4. 高性能解码 (Zero-Copy Decoder)
- [ ] **零拷贝解析**: 优化 `decoder.go`，使用 `binary.BigEndian.Uint16/32/64` 直接操作 `[]byte`。 <!-- id: zero-copy -->
- [ ] **全类型支持**: 确保所有 16 种 Modbus 数据格式（Float/Double/Long 变体）均有优化路径。 <!-- id: all-types -->

## 5. 数据缓冲与提交 (Data Buffering)
- [ ] **环形缓冲区**: 实现简单的 RingBuffer 替代 `[]model.Value` 切片，减少 GC。 <!-- id: ring-buffer -->
- [ ] **批量提交**: 实现 `flushLoop`，定时或定量提交数据。 <!-- id: batch-flush -->

## 6. 熔断与恢复 (Circuit Breaker)
- [ ] **点位级熔断**: `PointRuntime` 增加 `FailCount`，连续 3 次失败标记为 `Broken`，冷却 5 分钟。 <!-- id: point-cb -->
- [ ] **设备级熔断**: 统计设备周期成功率，低于 50% 触发降级（Interval * 2）。 <!-- id: device-cb -->

## 7. 监控指标 (Metrics)
- [ ] **指标埋点**: 在 `Read/Write` 关键路径增加 Counter 和 Histogram 统计。 <!-- id: metrics -->
- [ ] **健康度计算**: 定期计算并更新设备健康状态。 <!-- id: health -->

## 8. 集成与测试 (Integration)
- [ ] **集成测试**: 模拟网络波动，验证重连和降级逻辑。 <!-- id: integration-test -->
- [ ] **性能测试**: 对比优化前后的 CPU/内存和采集延迟。 <!-- id: perf-test -->
