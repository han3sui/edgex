<template>
  <a-card v-for="item in items" :key="item.id" class="northbound-card" hoverable>
    <template #title>
      <div class="card-title-row">
        <span class="protocol-tag">IoT 平台</span>
        <span class="card-name">{{ item.name || item.id }}</span>
      </div>
    </template>
    <template #extra>
      <a-space size="small">
        <a-tooltip content="统计">
          <a-button type="text" size="mini" @click="$emit('stats', item)">
            <template #icon><icon-bar-chart :size="14" /></template>
          </a-button>
        </a-tooltip>
        <a-tooltip content="配置">
          <a-button type="text" size="mini" @click="$emit('settings', item)">
            <template #icon><icon-settings :size="14" /></template>
          </a-button>
        </a-tooltip>
        <a-tooltip content="删除">
          <a-button type="text" size="mini" status="danger" @click="$emit('delete', 'iot-platform', item.id)">
            <template #icon><icon-delete :size="14" /></template>
          </a-button>
        </a-tooltip>
      </a-space>
    </template>

    <div class="card-info-list">
      <div class="info-row">
        <span class="info-label"><icon-cloud :size="14" /> Broker</span>
        <span class="info-value text-ellipsis">{{ item.broker }}</span>
      </div>
      <div class="info-row">
        <span class="info-label"><icon-user :size="14" /> Username</span>
        <span class="info-value text-ellipsis">{{ item.username || item.gateway_id || '-' }}</span>
      </div>
      <div class="info-row">
        <span class="info-label"><icon-apps :size="14" /> Product / Gateway</span>
        <span class="info-value text-ellipsis">{{ item.product_id || '-' }} / {{ item.gateway_id || '-' }}</span>
      </div>
      <div class="info-row">
        <span class="info-label"><icon-sync :size="14" /> 连接状态</span>
        <span class="info-value">
          <a-tag :color="statusColor(connectionStatus?.[item.id])" size="small">
            {{ statusText(connectionStatus?.[item.id]) }}
          </a-tag>
        </span>
      </div>
    </div>

    <template #actions>
      <a-tag :color="item.enable ? 'green' : 'gray'" size="small">
        {{ item.enable ? '启用' : '禁用' }}
      </a-tag>
    </template>
  </a-card>
</template>

<script setup>
import { IconSettings, IconDelete, IconCloud, IconApps, IconUser, IconSync, IconBarChart } from '@arco-design/web-vue/es/icon'

defineProps({
  items: { type: Array, default: () => [] },
  connectionStatus: { type: Object, default: () => ({}) }
})

defineEmits(['settings', 'delete', 'stats'])

const statusText = (s) => {
  if (s === 1) return '已连接'
  if (s === 2) return '重连中'
  if (s === 3) return '错误'
  return '未连接'
}

const statusColor = (s) => {
  if (s === 1) return 'green'
  if (s === 2) return 'orange'
  if (s === 3) return 'red'
  return 'gray'
}
</script>

<style scoped>
.northbound-card {
  border: 1px solid #e5e7eb;
  border-radius: 2px;
  margin-bottom: 16px;
  width: 100%;
  display: flex;
  flex-direction: column;
}

.card-info-list {
  display: flex;
  flex-direction: column;
  gap: 0;
  flex: 1;
  padding: 8px 0 0;
}

.northbound-card:hover {
  border-color: #0f172a;
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.protocol-tag {
  background: #f59e0b;
  color: #fff;
  font-family: monospace;
  font-size: 10px;
  padding: 0 4px;
  border-radius: 2px;
  line-height: 20px;
}

.card-name {
  font-size: 14px;
  font-weight: 500;
  color: #1e293b;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.info-row {
  width: 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
  font-size: 13px;
  border-bottom: 1px dashed #cbd5e1;
}

.info-row:last-child {
  border-bottom: none;
}

.info-label {
  color: #6b7280;
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.info-value {
  color: #334155;
  max-width: 60%;
  text-align: right;
}

.text-ellipsis {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
