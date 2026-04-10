<template>
  <a-modal
    v-model:visible="visible"
    title="IoT 平台对接配置"
    :width="680"
    @ok="saveSettings"
    :ok-loading="loading"
    unmount-on-close
    :footer="true"
    :mask-closable="false"
    class="industrial-modal"
  >
    <a-form :model="form" layout="horizontal" :label-col-props="{ span: 6 }" :wrapper-col-props="{ span: 18 }" class="industrial-form">
      <a-form-item label="通道名称" required>
        <a-input v-model="form.name" placeholder="例如: 生产环境 IoT 平台" />
      </a-form-item>

      <a-form-item label="启用状态">
        <a-switch v-model="form.enable" type="round" />
      </a-form-item>

      <a-divider orientation="left">MQTT 连接</a-divider>

      <a-form-item label="一键导入">
        <a-textarea
          v-model="importJson"
          placeholder='粘贴平台返回的连接信息 JSON，例如:&#10;{"clientId":"xxx","username":"xxx","passwd":"xxx","mqttHostUrl":"192.168.1.1","port":1883}'
          :auto-size="{ minRows: 2, maxRows: 4 }"
          class="mono-text"
        />
        <a-button size="small" style="margin-top: 6px" @click="parseImportJson">
          <template #icon><icon-import :size="14" /></template>
          解析并填入
        </a-button>
      </a-form-item>

      <a-form-item label="Broker 地址" required>
        <a-input v-model="form.broker" placeholder="tcp://192.168.1.1:1883" class="mono-text" />
      </a-form-item>

      <a-form-item label="Client ID" required>
        <a-input v-model="form.client_id" placeholder="MQTT Client ID（平台分配）" class="mono-text" />
      </a-form-item>

      <a-form-item label="Username" required>
        <a-input v-model="form.username" placeholder="MQTT 用户名（平台分配）" class="mono-text" />
      </a-form-item>

      <a-form-item label="Password" required>
        <a-input-password v-model="form.password" placeholder="MQTT 密码（平台分配）" class="mono-text" />
      </a-form-item>

      <a-divider orientation="left">平台标识</a-divider>

      <a-form-item label="Product ID" required>
        <a-input v-model="form.product_id" placeholder="平台产品 ID（用于构建 Topic）" class="mono-text" />
      </a-form-item>

      <a-form-item label="Gateway ID" required>
        <a-input v-model="form.gateway_id" placeholder="网关设备 ID（用于构建 Topic）" class="mono-text" />
      </a-form-item>

      <a-divider orientation="left">行为配置</a-divider>

      <a-form-item label="自动启动通道">
        <a-switch v-model="form.auto_start" type="round" />
        <span style="margin-left: 8px; color: #86909c; font-size: 12px">收到平台配置后自动启动采集通道</span>
      </a-form-item>
    </a-form>

    <template #footer>
      <div class="industrial-modal-footer">
        <a-button @click="visible = false" class="btn-secondary">取消</a-button>
        <a-button type="primary" :loading="loading" @click="saveSettings" class="btn-primary">
          <template #icon><icon-check /></template>保存配置
        </a-button>
      </div>
    </template>
  </a-modal>
</template>

<script setup>
import { ref, watch } from 'vue'
import { IconCheck, IconImport } from '@arco-design/web-vue/es/icon'
import { showMessage } from '@/composables/useGlobalState'
import request from '@/utils/request'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  config: { type: Object, default: null }
})

const emit = defineEmits(['update:modelValue', 'saved'])

const visible = ref(false)
const loading = ref(false)
const form = ref({})
const importJson = ref('')

watch(() => props.modelValue, (val) => {
  visible.value = val
})

watch(visible, (val) => {
  emit('update:modelValue', val)
  if (val) {
    importJson.value = ''
    if (props.config) {
      form.value = JSON.parse(JSON.stringify(props.config))
    } else {
      form.value = {
        id: '',
        name: 'IoT Platform',
        enable: true,
        broker: '',
        client_id: '',
        username: '',
        password: '',
        product_id: '',
        gateway_id: '',
        auto_start: true,
        cache: { enable: false, max_count: 0, flush_interval: '' }
      }
    }
  }
})

const parseImportJson = () => {
  try {
    const raw = importJson.value.trim()
    if (!raw) { showMessage('请先粘贴 JSON', 'warning'); return }
    const obj = JSON.parse(raw)

    if (obj.mqttHostUrl) {
      const host = obj.mqttHostUrl
      const port = obj.port || 1883
      form.value.broker = `tcp://${host}:${port}`
    }
    if (obj.clientId) form.value.client_id = obj.clientId
    if (obj.username) form.value.username = obj.username
    if (obj.passwd) form.value.password = obj.passwd
    if (obj.password) form.value.password = obj.password

    // Try to extract productID and gatewayID from username (format: "productID:gatewayID")
    if (obj.username && obj.username.includes(':')) {
      const parts = obj.username.split(':')
      if (!form.value.product_id) form.value.product_id = parts[0]
      if (!form.value.gateway_id) form.value.gateway_id = parts[1]
    }

    showMessage('解析成功，已填入连接信息', 'success')
    importJson.value = ''
  } catch (e) {
    showMessage('JSON 解析失败: ' + e.message, 'error')
  }
}

const saveSettings = async () => {
  if (!form.value.broker || !form.value.client_id || !form.value.username) {
    showMessage('请填写 Broker 地址、Client ID 和 Username', 'warning')
    return
  }
  if (!form.value.product_id || !form.value.gateway_id) {
    showMessage('请填写 Product ID 和 Gateway ID', 'warning')
    return
  }
  loading.value = true
  try {
    await request.post('/api/northbound/iot-platform', form.value)
    showMessage('IoT 平台配置已保存', 'success')
    visible.value = false
    emit('saved')
  } catch (e) {
    showMessage('保存失败: ' + e.message, 'error')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
:deep(.arco-modal) {
  border-radius: 0;
}

:deep(.arco-modal-header) {
  border-bottom: 1px solid #e5e7eb;
  height: 48px;
}

.industrial-form :deep(.arco-form-item-label) {
  font-weight: 500;
  color: #475569;
  font-size: 13px;
  white-space: nowrap;
}

.industrial-form :deep(.arco-input),
.industrial-form :deep(.arco-textarea),
.industrial-form :deep(.arco-select-view),
.industrial-form :deep(.arco-input-number) {
  border-radius: 0;
  background-color: #fcfcfc;
  border-color: #e5e7eb;
}

.mono-text {
  font-family: 'JetBrains Mono', 'Fira Code', monospace !important;
  font-size: 12px;
}

.industrial-modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 0 0;
}

.btn-primary {
  background-color: #0f172a !important;
  border-radius: 0 !important;
}

.btn-secondary {
  border-radius: 0 !important;
  border-color: #cbd5e1;
}

:deep(.arco-divider-horizontal) {
  margin: 16px 0;
  border-bottom-style: dashed;
}
</style>
