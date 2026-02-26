<template>
  <div>
    <!-- System Info -->
    <v-row>
      <v-col cols="12" md="3">
        <v-card class="glass-card pa-4" height="100%">
          <div class="text-overline mb-1">CPU 使用率</div>
          <div class="text-h4 font-weight-bold text-primary">{{ system.cpu_usage.toFixed(1) }}%</div>
          <v-progress-linear :model-value="system.cpu_usage" color="primary" height="4" class="mt-2"></v-progress-linear>
        </v-card>
      </v-col>
      <v-col cols="12" md="3">
        <v-card class="glass-card pa-4" height="100%">
          <div class="text-overline mb-1">内存使用</div>
          <div class="text-h4 font-weight-bold text-info">{{ system.memory_usage.toFixed(0) }} MB</div>
          <v-progress-linear :model-value="(system.memory_usage / 1024) * 100" color="info" height="4" class="mt-2"></v-progress-linear>
        </v-card>
      </v-col>
       <v-col cols="12" md="3">
        <v-card class="glass-card pa-4" height="100%">
          <div class="text-overline mb-1">协程数量</div>
          <div class="text-h4 font-weight-bold text-success">{{ system.goroutines }}</div>
        </v-card>
      </v-col>
       <v-col cols="12" md="3">
        <v-card class="glass-card pa-4" height="100%">
          <div class="text-overline mb-1">磁盘使用率</div>
          <div class="text-h4 font-weight-bold text-warning">{{ system.disk_usage.toFixed(1) }}%</div>
         <v-progress-linear :model-value="system.disk_usage" color="warning" height="4" class="mt-2"></v-progress-linear>
        </v-card>
      </v-col>
    </v-row>

    <!-- Southbound Channels with Metrics -->
    <v-row class="mt-4">
      <v-col cols="12">
        <div class="d-flex align-center mb-4">
          <div class="text-h6">采集通道</div>
          <v-spacer></v-spacer>
          <v-chip size="small" class="mr-2" color="success" variant="outlined">
            <v-icon size="x-small" start>mdi-check-circle</v-icon>
            在线: {{ totalOnlineDevices }}
          </v-chip>
          <v-chip size="small" color="error" variant="outlined">
            <v-icon size="x-small" start>mdi-alert-circle</v-icon>
            离线: {{ totalOfflineDevices }}
          </v-chip>
        </div>
      </v-col>
      
      <!-- Enhanced Channel Cards -->
      <v-col v-for="ch in channels" :key="ch.id" cols="12" md="6" lg="4">
        <v-card class="glass-card channel-card" hover @click="$router.push(`/channels/${ch.id}/devices`)">
          <!-- 通道头部信息 -->
          <v-card-item>
            <template v-slot:prepend>
              <div class="channel-icon mr-3">
                <v-icon :icon="getProtocolIcon(ch.protocol)" size="32" :color="getChannelStatusColor(ch)"></v-icon>
              </div>
            </template>
            <v-card-title class="d-flex align-center">
              {{ ch.name }}
              <v-chip size="x-small" :color="getQualityColor(ch.qualityScore)" variant="flat" class="ml-2">
                {{ ch.qualityScore || '-' }}
              </v-chip>
            </v-card-title>
            <v-card-subtitle>
              {{ ch.protocol }}
              <span class="mx-1">|</span>
              <v-chip size="x-small" :color="ch.enable ? 'success' : 'grey'" variant="text">
                {{ ch.enable ? '启用' : '禁用' }}
              </v-chip>
              <div v-if="getConnectionUrl(ch)" class="text-caption text-grey-darken-1 mt-1">
                <v-icon size="x-small">mdi-ip-network</v-icon>
                {{ getConnectionUrl(ch) }}
              </div>
            </v-card-subtitle>
            <template v-slot:append>
              <v-btn icon="mdi-chevron-right" variant="text" size="small"></v-btn>
            </template>
          </v-card-item>

          <!-- 关键指标 -->
          <v-card-text>
            <v-row dense class="channel-stats mb-3">
              <v-col cols="3">
                <div class="stat-item text-center">
                  <div class="text-caption text-grey-darken-1">设备</div>
                  <div class="text-h6 font-weight-bold">{{ ch.device_count || 0 }}</div>
                </div>
              </v-col>
              <v-col cols="3">
                <div class="stat-item text-center">
                  <div class="text-caption text-success">在线</div>
                  <div class="text-h6 font-weight-bold text-success">{{ ch.online_count || 0 }}</div>
                </div>
              </v-col>
              <v-col cols="3">
                <div class="stat-item text-center">
                  <div class="text-caption text-error">离线</div>
                  <div class="text-h6 font-weight-bold text-error">{{ ch.offline_count || 0 }}</div>
                </div>
              </v-col>
              <v-col cols="3">
                <div class="stat-item text-center">
                  <div class="text-caption text-grey-darken-1">成功率</div>
                  <div class="text-h6 font-weight-bold" :class="getSuccessRateColor(ch.successRate)">
                    {{ formatPercent(ch.successRate) }}
                  </div>
                </div>
              </v-col>
            </v-row>

            <!-- 通信质量指标 (如果有) -->
            <div v-if="ch.metrics" class="metrics-section">
              <div class="d-flex align-center mb-2" style="position: relative; height: 24px;">
                <div class="d-flex align-center">
                  <v-icon size="x-small" class="mr-1 text-info">mdi-pulse</v-icon>
                  <span class="text-caption text-grey-darken-1">通信质量</span>
                </div>
                
                <!-- 重连次数放在中间 -->
                <div v-if="ch.metrics.reconnectCount > 0" 
                  class="text-caption text-info font-weight-bold" 
                  style="position: absolute; left: 50%; transform: translateX(-50%); white-space: nowrap;"
                >
                  <v-icon size="x-small" class="mr-1">mdi-refresh</v-icon>
                  重连: {{ ch.metrics.reconnectCount }}
                </div>
                
                <v-spacer></v-spacer>
                <span class="text-caption">RTT: {{ formatDuration(ch.metrics.avgRtt) }}</span>
              </div>
              
              <!-- 成功率进度条 -->
              <v-progress-linear
                :model-value="(ch.metrics.successRate || 0) * 100"
                :color="getSuccessRateBarColor(ch.metrics.successRate)"
                height="6"
                rounded
                class="mb-2"
              ></v-progress-linear>
              
              <!-- 错误统计 -->
              <div class="d-flex justify-center text-caption">
                <span v-if="ch.metrics.timeoutCount > 0" class="text-warning mx-1">
                  <v-icon size="x-small">mdi-timer-off</v-icon>
                  超时: {{ ch.metrics.timeoutCount }}
                </span>
                <span v-if="ch.metrics.crcError > 0" class="text-error mx-1">
                  <v-icon size="x-small">mdi-alert-circle</v-icon>
                  CRC: {{ ch.metrics.crcError }}
                </span>
              </div>
            </div>

            <!-- 最后采集时间 -->
            <div v-if="ch.last_collect_time" class="mt-2 text-caption text-grey-darken-1 text-right">
              <v-icon size="x-small">mdi-clock-outline</v-icon>
              {{ formatTime(ch.last_collect_time) }}
            </div>
          </v-card-text>
        </v-card>
      </v-col>
      
      <v-col v-if="channels.length === 0" cols="12">
        <v-alert type="info" variant="tonal" class="glass-card">
          暂无采集通道配置。 <router-link to="/channels">添加通道</router-link>.
        </v-alert>
      </v-col>
    </v-row>

    <!-- Northbound -->
    <v-row class="mt-4">
       <v-col cols="12">
        <div class="text-h6 mb-4">北向数据上报</div>
      </v-col>
      <v-col v-for="nb in northbound" :key="nb.id" cols="12" md="4">
         <v-card class="glass-card">
            <v-card-title class="d-flex justify-space-between align-center">
                {{ nb.name }}
                <v-chip size="small" :color="nb.status === 'Running' ? 'success' : (nb.status === 'Disabled' ? 'grey' : 'error')">{{ nb.status }}</v-chip>
            </v-card-title>
             <v-card-subtitle>{{ nb.type }}</v-card-subtitle>
             <v-card-actions>
                 <v-spacer></v-spacer>
                 <v-btn variant="text" color="primary" to="/northbound">配置</v-btn>
             </v-card-actions>
         </v-card>
      </v-col>
       <v-col v-if="northbound.length === 0" cols="12">
          <v-alert type="info" variant="tonal" class="glass-card">
              暂无北向数据上报配置。 <router-link to="/northbound">配置北向</router-link>.
          </v-alert>
      </v-col>
    </v-row>
    
    <!-- Edge Compute Stats Summary -->
    <v-row class="mt-4">
         <v-col cols="12">
            <div class="text-h6 mb-4">边缘计算状态</div>
            <v-card class="glass-card pa-4" @click="$router.push('/edge-compute/metrics')" hover>
                <v-row>
                    <v-col cols="6" md="3">
                        <div class="text-caption">规则数</div>
                        <div class="text-h5">{{ edgeRules.rule_count || 0 }}</div>
                    </v-col>
                     <v-col cols="6" md="3">
                        <div class="text-caption">已触发</div>
                        <div class="text-h5 text-primary">{{ edgeRules.rules_triggered || 0 }}</div>
                    </v-col>
                     <v-col cols="6" md="3">
                        <div class="text-caption">已执行</div>
                        <div class="text-h5 text-success">{{ edgeRules.rules_executed || 0 }}</div>
                    </v-col>
                     <v-col cols="6" md="3">
                        <div class="text-caption">工作池负载</div>
                         <v-progress-linear 
                            :model-value="(edgeRules.worker_pool_usage / (edgeRules.worker_pool_size || 1)) * 100" 
                            color="warning" height="10" striped class="mt-1">
                         </v-progress-linear>
                    </v-col>
                </v-row>
            </v-card>
         </v-col>
    </v-row>

  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import request from '@/utils/request'

const system = ref({
    cpu_usage: 0,
    memory_usage: 0,
    disk_usage: 0,
    goroutines: 0
})
const channels = ref([])
const northbound = ref([])
const edgeRules = ref({})

let timer = null

// 计算总在线/离线设备数
const totalOnlineDevices = computed(() => {
  return channels.value.reduce((sum, ch) => sum + (ch.online_count || 0), 0)
})

const totalOfflineDevices = computed(() => {
  return channels.value.reduce((sum, ch) => sum + (ch.offline_count || 0), 0)
})

// 获取连接地址
const getConnectionUrl = (channel) => {
    if (!channel || !channel.config) return null
    const cfg = channel.config
    
    // TCP 协议
    if (channel.protocol?.includes('tcp')) {
        if (cfg.url) {
            const match = cfg.url.match(/tcp:\/\/(.+):(\d+)/)
            if (match) return `${match[1]}:${match[2]}`
            return cfg.url
        }
        if (cfg.address) return cfg.address
        if (cfg.ip) return `${cfg.ip}:${cfg.port || 502}`
    }
    
    // RTU Over TCP
    if (channel.protocol === 'modbus-rtu-over-tcp') {
        if (cfg.url) {
            const match = cfg.url.match(/tcp:\/\/(.+):(\d+)/)
            if (match) return `${match[1]}:${match[2]} (RTU)`
        }
    }
    
    // OPC UA
    if (channel.protocol === 'opc-ua') {
        return cfg.url || cfg.endpoint
    }
    
    // BACnet
    if (channel.protocol === 'bacnet-ip') {
        return `${cfg.ip || '0.0.0.0'}:${cfg.port || 47808}`
    }
    
    // 串口
    if (channel.protocol?.includes('rtu') || channel.protocol === 'dlt645') {
        if (cfg.port) return cfg.port
    }
    
    return null
}

// 获取协议图标
const getProtocolIcon = (protocol) => {
  const icons = {
    'modbus-tcp': 'mdi-lan-connect',
    'modbus-rtu': 'mdi-serial-port',
    'modbus-rtu-over-tcp': 'mdi-lan-connect',
    'bacnet-ip': 'mdi-ip-network',
    'opc-ua': 'mdi-server',
    'dlt645': 'mdi-meter-electric',
    's7': 'mdi-cpu-64-bit',
    'ethernet-ip': 'mdi-ethernet',
    'mitsubishi-slmp': 'mdi-ladder',
    'omron-fins': 'mdi-ladder'
  }
  return icons[protocol] || 'mdi-connection'
}

// 通道状态颜色
const getChannelStatusColor = (ch) => {
  if (!ch.enable) return 'grey'
  if (ch.offline_count > 0 && ch.online_count === 0) return 'error'
  if (ch.offline_count > 0) return 'warning'
  if (ch.qualityScore >= 90) return 'success'
  if (ch.qualityScore >= 60) return 'warning'
  return 'error'
}

// 质量评分颜色 (工业标准分级)
const getQualityColor = (score) => {
  if (score === undefined || score === null || score === 0) return 'grey'
  if (score === 100) return 'primary'    // 完美 (蓝色)
  if (score >= 90) return 'success'     // 优秀 (绿色)
  if (score >= 80) return 'warning'     // 良好 (橙色/黄色)
  return 'error'                         // 30-79% 为警告 (红色)
}

// 成功率相关
const getSuccessRateColor = (rate) => {
  if (!rate && rate !== 0) return 'text-grey'
  if (rate >= 0.99) return 'text-success'
  if (rate >= 0.95) return 'text-warning'
  return 'text-error'
}

const getSuccessRateBarColor = (rate) => {
  if (rate === undefined || rate === null) return 'grey'
  const score = rate * 100
  if (score === 100) return 'primary'    // 100% 蓝色
  if (score >= 90) return 'success'     // 90% 绿色
  if (score >= 80) return 'warning'     // 80% 橙色
  if (score > 0) return 'error'         // 0-80% 红色
  return 'grey'                         // 0% 灰色
}

// 格式化
const formatPercent = (val) => {
  if (val === undefined || val === null) return '-'
  return (val * 100).toFixed(0) + '%'
}

const formatDuration = (ms) => {
  if (ms === undefined || ms === null) return '-'
  if (ms < 1) return '<1ms'
  if (ms < 1000) return ms.toFixed(2) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}

const formatTime = (ts) => {
  if (!ts) return ''
  const date = new Date(ts)
  const now = new Date()
  const diff = now - date
  
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)}分钟前`
  return date.toLocaleTimeString()
}

const fetchData = async () => {
    try {
        const data = await request.get('/api/dashboard/summary')
        system.value = data.system
        
        // 处理通道数据，合并metrics
        channels.value = (data.channels || []).map(ch => {
          // 计算质量评分
          let qualityScore = 100
          if (ch.metrics) {
            const m = ch.metrics
            if (m.successRate !== undefined) qualityScore -= (1 - m.successRate) * 40
            if (m.crcErrorRate !== undefined) qualityScore -= m.crcErrorRate * 20
            if (m.retryRate !== undefined) qualityScore -= m.retryRate * 20
            qualityScore = Math.max(0, Math.round(qualityScore))
          }
          
          return {
            ...ch,
            qualityScore,
            successRate: ch.metrics?.successRate || ch.success_rate || 0
          }
        }).sort((a, b) => a.name.localeCompare(b.name))
        
        northbound.value = data.northbound || []
        edgeRules.value = data.edge_rules || {}
    } catch (e) {
        console.error(e)
    }
}

onMounted(() => {
    fetchData()
    timer = setInterval(fetchData, 2000)
})

onUnmounted(() => {
    if (timer) clearInterval(timer)
})
</script>

<style scoped>
.channel-card {
  transition: all 0.3s ease;
  border-left: 3px solid transparent;
}

.channel-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.channel-card:hover .channel-icon {
  transform: scale(1.1);
}

.channel-icon {
  transition: transform 0.3s ease;
}

.channel-stats {
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding-top: 12px;
}

.stat-item {
  transition: background 0.2s;
  border-radius: 4px;
  padding: 4px;
}

.stat-item:hover {
  background: rgba(255, 255, 255, 0.05);
}

.metrics-section {
  background: rgba(0, 0, 0, 0.2);
  border-radius: 8px;
  padding: 12px;
}
</style>
