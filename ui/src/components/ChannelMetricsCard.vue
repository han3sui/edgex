<template>
  <v-card class="glass-card" :class="{ 'metrics-card': true, 'expanded': showDetails }">
    <v-card-text class="pa-4">

      <!-- 顶部：大仪表 + 状态信息 -->
      <div class="d-flex align-center mb-4">

        <!-- 🔵 大圆形质量仪表 -->
        <div class="quality-score-wrapper mr-6">
          <v-progress-circular
            :model-value="qualityScore"
            :color="getQualityColor(qualityScore)"
            :size="120"
            :width="10"
            bg-color="grey-darken-3"
            class="quality-ring"
          >
            <div class="quality-inner">
              <div
                class="quality-value"
                :class="`text-${getQualityColor(qualityScore)}`"
              >
                {{ qualityScore }}
              </div>
              <div class="quality-label">
                质量评分
              </div>
              <div
                class="quality-level"
                :class="`text-${getQualityColor(qualityScore)}`"
              >
                {{ getQualityLabel(qualityScore) }}
              </div>
            </div>
          </v-progress-circular>
        </div>

        <!-- 右侧状态信息 -->
        <div class="flex-grow-1">

          <div class="d-flex align-center mb-2">
            <v-chip
              size="small"
              :color="getQualityColor(qualityScore)"
              variant="flat"
              class="font-weight-medium"
            >
              通道状态: {{ getQualityLabel(qualityScore) }}
            </v-chip>

            <span v-if="metrics?.reconnectCount > 0" class="text-caption text-warning ml-3">
              <v-icon size="x-small">mdi-refresh-alert</v-icon>
              重连 {{ metrics.reconnectCount }} 次
            </span>
          </div>

          <div class="text-caption text-grey-darken-1 mb-1">
            <v-icon size="x-small">mdi-clock-outline</v-icon>
            {{ connectionDuration }}
          </div>

          <div class="text-caption text-grey-darken-1">
            <v-icon size="x-small">mdi-lan-connect</v-icon>
            本地 {{ metrics?.localIp || '-' }}:{{ metrics?.localPort || '-' }}
            →
            目标 {{ metrics?.remoteIp || '-' }}:{{ metrics?.remotePort || '-' }}
          </div>
        </div>

        <!-- 展开按钮 -->
        <v-btn
          size="small"
          variant="text"
          :icon="showDetails ? 'mdi-chevron-up' : 'mdi-chevron-down'"
          @click="showDetails = !showDetails"
        />
      </div>

      <!-- 核心指标 -->
      <v-row dense class="metrics-summary">
        <v-col cols="4">
          <div class="metric-item text-center">
            <div class="text-caption text-grey-darken-1">成功率</div>
            <div
              class="text-body-1 font-weight-bold"
              :class="getSuccessRateColor(metrics?.successRate)"
            >
              {{ formatPercent(metrics?.successRate) }}
            </div>
          </div>
        </v-col>

        <v-col cols="4">
          <div class="metric-item text-center">
            <div class="text-caption text-grey-darken-1">平均 RTT</div>
            <div class="text-body-1 font-weight-bold">
              {{ formatDuration(metrics?.avgRtt) }}
            </div>
          </div>
        </v-col>

        <v-col cols="4">
          <div class="metric-item text-center">
            <div class="text-caption text-grey-darken-1">丢包率</div>
            <div
              class="text-body-1 font-weight-bold"
              :class="getPacketLossColor(metrics?.packetLoss)"
            >
              {{ formatPercent(metrics?.packetLoss) }}
            </div>
          </div>
        </v-col>
      </v-row>

    </v-card-text>
  </v-card>
</template>

<script setup>
import { ref, computed } from 'vue'

const props = defineProps({
  metrics: {
    type: Object,
    default: () => ({})
  }
})

const showDetails = ref(false)

/* =======================
   质量评分计算（100分制）
======================= */
const qualityScore = computed(() => {
  if (!props.metrics) return 100

  let score = 100
  const m = props.metrics

  if (m.successRate !== undefined)
    score -= (1 - m.successRate) * 40

  if (m.crcErrorRate !== undefined)
    score -= m.crcErrorRate * 20

  if (m.retryRate !== undefined)
    score -= m.retryRate * 20

  if (m.avgRtt > 100)
    score -= Math.min(10, (m.avgRtt - 100) / 50)

  return Math.max(0, Math.round(score))
})

/* =======================
   连接时长
======================= */
const connectionDuration = computed(() => {
  const seconds = props.metrics?.connectionSeconds || 0
  if (seconds < 60) return `已连接 ${seconds}s`
  if (seconds < 3600) return `已连接 ${Math.floor(seconds / 60)}m`
  return `已连接 ${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
})

/* =======================
   质量等级
======================= */
const getQualityLabel = (score) => {
  if (score >= 90) return 'Excellent'
  if (score >= 75) return 'Good'
  if (score >= 60) return 'Unstable'
  return 'Poor'
}

const getQualityColor = (score) => {
  if (score >= 90) return 'success'
  if (score >= 75) return 'info'
  if (score >= 60) return 'warning'
  return 'error'
}

/* =======================
   颜色规则
======================= */
const getSuccessRateColor = (rate) => {
  if (rate >= 0.99) return 'text-success'
  if (rate >= 0.95) return 'text-warning'
  return 'text-error'
}

const getPacketLossColor = (rate) => {
  if (rate < 0.01) return 'text-success'
  if (rate < 0.05) return 'text-warning'
  return 'text-error'
}

/* =======================
   格式化
======================= */
const formatPercent = (val) => {
  if (val === undefined || val === null) return '-'
  return (val * 100).toFixed(1) + '%'
}

const formatDuration = (ms) => {
  if (ms === undefined || ms === null) return '-'
  if (ms < 1) return '<1ms'
  if (ms < 1000) return ms.toFixed(2) + 'ms'
  return (ms / 1000).toFixed(2) + 's'
}
</script>

<style scoped>
.metrics-card {
  transition: all 0.3s ease;
}

.quality-score-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
}

.quality-ring {
  filter: drop-shadow(0 6px 14px rgba(0, 0, 0, 0.2));
  transition: all 0.4s ease;
}

.quality-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  line-height: 1.1;
}

.quality-value {
  font-size: 32px;
  font-weight: 700;
}

.quality-label {
  font-size: 12px;
  opacity: 0.65;
  margin-top: 2px;
}

.quality-level {
  font-size: 14px;
  font-weight: 600;
  margin-top: 4px;
}

.metrics-summary {
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding-top: 12px;
}

.metric-item {
  padding: 6px;
  border-radius: 6px;
  transition: background 0.2s;
}

.metric-item:hover {
  background: rgba(255, 255, 255, 0.05);
}
</style>