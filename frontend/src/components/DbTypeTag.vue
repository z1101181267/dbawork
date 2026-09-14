<script setup>
import { computed } from 'vue'

const props = defineProps({
  value: { type: String, default: '' },
  label: { type: String, default: '' }
})

// 品牌色映射（未列的走稳定哈希调色板）
const COLOR_MAP = {
  mysql: '#2f7bd9',
  mariadb: '#3a8fb7',
  oracle: '#c74634',
  postgresql: '#336791',
  mssql: '#a33ea1',
  sqlite: '#0f80cc',
  dm: '#b45a2b',
  kingbase: '#1f7a8c',
  gaussdb: '#d14f4f',
  opengauss: '#c0392b',
  oceanbase: '#1677b8',
  tidb: '#d4472b',
  redis: '#c0392b',
  mongodb: '#4db33d',
  clickhouse: '#f2c811',
  hive: '#f5a623',
  doris: '#3f78c9',
  starrocks: '#5b6eae'
}

const PALETTE = ['#5470c6', '#91cc75', '#fac858', '#ee6666', '#73c0de', '#3ba272', '#fc8452', '#9a60b4']

const color = computed(() => {
  if (!props.value) return '#909399'
  if (COLOR_MAP[props.value]) return COLOR_MAP[props.value]
  let h = 0
  for (const ch of props.value) h = (h * 31 + ch.charCodeAt(0)) % 997
  return PALETTE[h % PALETTE.length]
})
</script>

<template>
  <el-tag
    size="small"
    effect="plain"
    :style="{ color, borderColor: color, backgroundColor: color + '14' }"
  >
    {{ label || value || '—' }}
  </el-tag>
</template>
