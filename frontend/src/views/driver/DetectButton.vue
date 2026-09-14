<script setup>
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { detectOne } from '../../api/driver'

const props = defineProps({
  dbType: { type: String, required: true }
})
const emit = defineEmits(['done'])

const loading = ref(false)

async function onDetect() {
  loading.value = true
  try {
    const r = await detectOne(props.dbType)
    if (r && r.installed) {
      ElMessage.success(`${props.dbType}：已安装${r.installed_version ? `（${r.installed_version}）` : ''}`)
    } else {
      ElMessage.warning(`${props.dbType}：未安装${r?.note ? `（${r.note}）` : ''}`)
    }
    emit('done')
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-button size="small" text type="primary" :loading="loading" @click="onDetect">检测</el-button>
</template>
