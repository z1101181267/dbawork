<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Download, UploadFilled } from '@element-plus/icons-vue'
import { importCsv } from '../../api/datasource'

const props = defineProps({
  modelValue: { type: Boolean, default: false }
})
const emit = defineEmits(['update:modelValue', 'imported'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const file = ref(null)
const importing = ref(false)
const result = ref(null)

function onFileChange(uploadFile) {
  file.value = uploadFile?.raw || null
  result.value = null
}

function onFileRemove() {
  file.value = null
}

function downloadTemplate() {
  const header = 'db_type,name,host,port,db_name,username,password,group_name'
  const example = 'mysql,示例库,127.0.0.1,3306,testdb,root,your_password,生产库'
  const csv = '\uFEFF' + header + '\n' + example + '\n'
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'datasource_template.csv'
  a.click()
  URL.revokeObjectURL(a.href)
}

async function onImport() {
  if (!file.value) {
    ElMessage.warning('请先选择 CSV 文件')
    return
  }
  importing.value = true
  result.value = null
  try {
    const fd = new FormData()
    fd.append('file', file.value)
    result.value = await importCsv(fd)
    if (result.value.success_count > 0) {
      emit('imported')
    }
  } catch {
    /* 拦截器已提示 */
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <el-dialog v-model="visible" title="批量导入数据源（CSV）" width="640px">
    <div class="hint">
      <p>列头格式：<code>db_type, name, host, port, db_name, username, password, group_name</code></p>
      <p class="muted">
        可选列：<code>tunnel_type, tunnel_host, tunnel_port, tunnel_user, tunnel_password</code>；
        支持别名（user/database/pwd 等），group_name 不存在时会自动创建分组。
      </p>
      <el-button link type="primary" :icon="Download" @click="downloadTemplate">下载 CSV 模板</el-button>
    </div>

    <el-upload
      drag
      accept=".csv"
      :auto-upload="false"
      :limit="1"
      :on-change="onFileChange"
      :on-remove="onFileRemove"
      class="uploader"
    >
      <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
      <div class="el-upload__text">拖拽 CSV 文件到此处，或 <em>点击选择</em></div>
    </el-upload>

    <el-alert
      v-if="result"
      class="result"
      :type="result.fail_count ? 'warning' : 'success'"
      :closable="false"
      show-icon
      :title="`导入完成：成功 ${result.success_count} 条，失败 ${result.fail_count} 条`"
    />
    <el-table
      v-if="result && result.errors && result.errors.length"
      :data="result.errors"
      size="small"
      max-height="200"
      class="result"
    >
      <el-table-column prop="line" label="行号" width="80" />
      <el-table-column prop="message" label="失败原因" />
    </el-table>

    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
      <el-button type="primary" :loading="importing" @click="onImport">开始导入</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.hint p {
  margin: 0 0 6px;
  font-size: 13px;
}

.hint code {
  background: #f4f4f5;
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 12px;
}

.muted {
  color: #909399;
}

.uploader {
  margin: 14px 0;
}

.result {
  margin-top: 12px;
}
</style>
