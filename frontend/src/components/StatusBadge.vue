<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: Number, default: 0 },
  lastError: { type: String, default: '' }
})

const CONF = {
  0: { type: 'info', text: '未测' },
  1: { type: 'success', text: '正常' },
  2: { type: 'danger', text: '异常' }
}

const conf = computed(() => CONF[props.status] || { type: 'info', text: '未知' })
</script>

<template>
  <el-tooltip
    :content="lastError || '暂无错误信息'"
    placement="top"
    :disabled="!lastError"
  >
    <el-tag :type="conf.type" size="small" effect="light">{{ conf.text }}</el-tag>
  </el-tooltip>
</template>
