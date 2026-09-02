<template>
  <div class="page-container">
    <div class="toolbar">
      <div class="toolbar-left">
        <el-button @click="$router.back()">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
        <h2>设备控制 - {{ deviceId }}</h2>
      </div>
    </div>

    <el-row :gutter="20">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>下发控制指令</span>
          </template>
          <el-form label-width="100px" @submit.prevent="handleControl">
            <el-form-item label="控制对象" v-for="(item, index) in controlItems" :key="index">
              <div style="display: flex; gap: 8px; width: 100%;">
                <el-input v-model="item.key" placeholder="标签键" style="flex: 1" />
                <el-input-number v-model="item.value" :min="0" :max="65535" placeholder="值" style="width: 120px" />
                <el-button type="danger" circle size="small" @click="removeItem(index)">
                  <el-icon><Delete /></el-icon>
                </el-button>
              </div>
            </el-form-item>
            <el-form-item>
              <el-button @click="addItem">添加控制项</el-button>
              <el-button type="primary" native-type="submit" :loading="sending">发送指令</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>

      <el-col :span="12">
        <el-card>
          <template #header>
            <span>快捷控制</span>
          </template>
          <div class="quick-controls">
            <el-button
              v-for="btn in quickButtons"
              :key="btn.label"
              :type="btn.type as any"
              style="margin: 8px;"
              :loading="sendingBtn === btn.label"
              @click="quickControl(btn)"
            >
              {{ btn.label }}
            </el-button>
          </div>
          <div v-if="lastResult" style="margin-top: 16px;">
            <el-alert :title="`指令已发送，消息ID: ${lastResult}`" type="success" :closable="false" show-icon />
          </div>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Delete } from '@element-plus/icons-vue'
import { sendDeviceControl } from '@/api/device'

const route = useRoute()
const deviceId = String(route.params.deviceId)

const sending = ref(false)
const sendingBtn = ref('')
const lastResult = ref('')

interface ControlItem {
  key: string
  value: number
}

const controlItems = reactive<ControlItem[]>([
  { key: 'relay1', value: 0 }
])

const quickButtons = [
  { label: '继电器1 开', key: 'relay1', value: 1, type: 'success' },
  { label: '继电器1 关', key: 'relay1', value: 0, type: 'danger' },
  { label: '继电器2 开', key: 'relay2', value: 1, type: 'success' },
  { label: '继电器2 关', key: 'relay2', value: 0, type: 'danger' },
  { label: '重启设备', key: 'reboot', value: 1, type: 'warning' },
  { label: '恢复出厂', key: 'factory_reset', value: 1, type: 'danger' }
]

function addItem() {
  controlItems.push({ key: '', value: 0 })
}

function removeItem(index: number) {
  if (controlItems.length <= 1) {
    ElMessage.warning('至少保留一个控制项')
    return
  }
  controlItems.splice(index, 1)
}

function buildTags(): Record<string, number> {
  const tags: Record<string, number> = {}
  for (const item of controlItems) {
    if (item.key.trim()) {
      tags[item.key.trim()] = item.value
    }
  }
  return tags
}

async function handleControl() {
  const tags = buildTags()
  if (Object.keys(tags).length === 0) {
    ElMessage.warning('请输入至少一个有效的标签键')
    return
  }

  sending.value = true
  try {
    const res = await sendDeviceControl(deviceId, tags)
    lastResult.value = res.data.msg_id
    ElMessage.success(`控制指令发送成功，消息ID: ${res.data.msg_id}`)
  } catch {
  } finally {
    sending.value = false
  }
}

async function quickControl(btn: { label: string; key: string; value: number }) {
  sendingBtn.value = btn.label
  try {
    const res = await sendDeviceControl(deviceId, { [btn.key]: btn.value })
    lastResult.value = res.data.msg_id
    ElMessage.success(`${btn.label} 指令发送成功，消息ID: ${res.data.msg_id}`)
  } catch {
  } finally {
    sendingBtn.value = ''
  }
}
</script>

<style scoped>
.quick-controls {
  display: flex;
  flex-wrap: wrap;
}
</style>
