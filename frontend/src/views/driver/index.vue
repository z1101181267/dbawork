<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Refresh, UploadFilled } from '@element-plus/icons-vue'
import DbTypeTag from '../../components/DbTypeTag.vue'
import DetectButton from './DetectButton.vue'
import InstallDialog from './InstallDialog.vue'
import { activateDriver, detectAll, listDbTypes, listDrivers, removeDriver, uninstallDriver, uploadJar } from '../../api/driver'

const drivers = ref([])
const dbTypes = ref([])
const loading = ref(false)
const detectingAll = ref(false)
const filterType = ref('')

const installVisible = ref(false)
const installRow = ref(null)

const uploadVisible = ref(false)
const uploading = ref(false)
const uploadForm = reactive({ db_type: '', version: '', driver_class: '', file: null })

const typeLabelMap = computed(() => {
  const m = {}
  for (const t of dbTypes.value) m[t.key] = t.name_zh
  return m
})
const typeLabel = (key) => typeLabelMap.value[key] || key

// 按类型目录顺序排序（catalog order），同类型内 python 在前，再按 ID
const sortedDrivers = computed(() => {
  const orderMap = {}
  dbTypes.value.forEach((t, i) => (orderMap[t.key] = i))
  return [...drivers.value].sort((a, b) => {
    const oa = orderMap[a.db_type] ?? 999
    const ob = orderMap[b.db_type] ?? 999
    if (oa !== ob) return oa - ob
    if (a.driver_kind !== b.driver_kind) return a.driver_kind === 'python' ? -1 : 1
    return a.id - b.id
  })
})

async function load() {
  loading.value = true
  try {
    drivers.value = await listDrivers({ db_type: filterType.value || undefined })
  } finally {
    loading.value = false
  }
}

async function onDetectAll() {
  detectingAll.value = true
  try {
    const r = await detectAll()
    ElMessage.success(`全量检测完成：已安装 ${r.installed} / ${r.total}`)
    await load()
  } catch {
    /* 拦截器已提示 */
  } finally {
    detectingAll.value = false
  }
}

function openInstall(row) {
  installRow.value = row
  installVisible.value = true
}

async function onUninstall(row) {
  await ElMessageBox.confirm(
    `确定卸载 ${row.db_type} 的驱动「${row.package_name}」吗？（从 Python 环境 pip uninstall）`,
    '卸载确认',
    { type: 'warning', confirmButtonText: '卸载', cancelButtonText: '取消' }
  )
  const r = await uninstallDriver({ db_type: row.db_type, driver_kind: row.driver_kind })
  if (r.ok) ElMessage.success(r.message || '卸载完成')
  else ElMessage.warning(r.message || '卸载未完成')
  await load()
}

async function onActivate(row) {
  const r = await activateDriver(row.id)
  if (r && r.sync) ElMessage.warning(`默认已设置（${r.sync}）`)
  else ElMessage.success('已设为默认驱动')
  await load()
}

async function onDelete(row) {
  await ElMessageBox.confirm(
    row.driver_kind === 'jdbc'
      ? `确定删除驱动「${row.jar_filename}」吗？对应的 JAR 文件也会被删除。`
      : `确定删除驱动记录「${row.db_type} / ${row.package_name || row.driver_kind}」吗？`,
    '删除确认',
    { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' }
  )
  await removeDriver(row.id)
  ElMessage.success('已删除')
  await load()
}

function openUpload(row = null) {
  uploadForm.db_type = row?.db_type || ''
  uploadForm.version = row?.version || ''
  uploadForm.driver_class = row?.driver_class || ''
  uploadForm.file = null
  uploadVisible.value = true
}

function onJarFileChange(uploadFile) {
  uploadForm.file = uploadFile?.raw || null
}

async function onUpload() {
  if (!uploadForm.db_type) return ElMessage.warning('请选择数据库类型')
  if (!uploadForm.file) return ElMessage.warning('请选择 JAR 文件')
  uploading.value = true
  try {
    const fd = new FormData()
    fd.append('file', uploadForm.file)
    fd.append('db_type', uploadForm.db_type)
    fd.append('version', uploadForm.version || '')
    fd.append('driver_class', uploadForm.driver_class || '')
    await uploadJar(fd)
    ElMessage.success('JAR 上传成功')
    uploadVisible.value = false
    await load()
  } finally {
    uploading.value = false
  }
}

onMounted(async () => {
  dbTypes.value = await listDbTypes().catch(() => [])
  await load()
})
</script>

<template>
  <el-card shadow="never">
    <div class="toolbar">
      <div class="left">
        <el-select v-model="filterType" placeholder="按类型筛选" clearable style="width: 190px" @change="load">
          <el-option v-for="t in dbTypes" :key="t.key" :label="t.name_zh" :value="t.key" />
        </el-select>
        <el-button :icon="Refresh" :loading="detectingAll" type="primary" @click="onDetectAll">全量检测</el-button>
      </div>
      <div class="right">
        <el-button :icon="UploadFilled" @click="openUpload()">上传 JAR</el-button>
      </div>
    </div>

    <el-table v-loading="loading" :data="sortedDrivers" stripe empty-text="暂无驱动记录">
      <el-table-column prop="id" label="ID" width="64" />
      <el-table-column label="DB 类型" width="150">
        <template #default="{ row }">
          <DbTypeTag :value="row.db_type" :label="typeLabel(row.db_type)" />
        </template>
      </el-table-column>
      <el-table-column label="驱动种类" width="100">
        <template #default="{ row }">
          <el-tag size="small" :type="row.driver_kind === 'python' ? 'success' : 'warning'" effect="plain">
            {{ row.driver_kind }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="包名 / JAR 文件" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.driver_kind === 'python' ? row.package_name : row.jar_filename || '—' }}
        </template>
      </el-table-column>
      <el-table-column label="版本" width="100">
        <template #default="{ row }">
          {{ row.driver_kind === 'python' ? row.installed_version || '—' : row.version || '—' }}
        </template>
      </el-table-column>
      <el-table-column label="可用状态" width="150">
        <template #default="{ row }">
          <el-tag v-if="row.installed" type="success" size="small" effect="light">
            已安装{{ row.installed_version ? ` · ${row.installed_version}` : '' }}
          </el-tag>
          <el-tag v-else type="info" size="small" effect="light">未安装</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="默认" width="90">
        <template #default="{ row }">
          <el-tag v-if="row.is_active" type="danger" size="small" effect="light">默认</el-tag>
          <el-button v-else size="small" text type="primary" :disabled="!row.installed" @click="onActivate(row)">
            设为默认
          </el-button>
        </template>
      </el-table-column>
      <el-table-column prop="note" label="备注" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">{{ row.note || '—' }}</template>
      </el-table-column>
      <el-table-column label="操作" width="290" fixed="right">
        <template #default="{ row }">
          <template v-if="row.driver_kind === 'python'">
            <DetectButton :db-type="row.db_type" @done="load" />
            <el-button size="small" text type="primary" @click="openInstall(row)">安装</el-button>
            <el-button
              size="small"
              text
              :disabled="row.package_name === 'sqlite3' || !row.installed"
              @click="onUninstall(row)"
            >
              卸载
            </el-button>
          </template>
          <template v-else>
            <el-button size="small" text type="primary" @click="openUpload(row)">上传新版本</el-button>
          </template>
          <el-button size="small" text type="danger" @click="onDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <InstallDialog v-model="installVisible" :row="installRow" @saved="load" />

  <el-dialog v-model="uploadVisible" title="上传 JDBC 驱动（JAR）" width="560px">
    <el-form label-position="top">
      <el-form-item label="数据库类型" required>
        <el-select v-model="uploadForm.db_type" placeholder="选择类型" style="width: 100%">
          <el-option v-for="t in dbTypes" :key="t.key" :label="`${t.name_zh}（${t.key}）`" :value="t.key" />
        </el-select>
      </el-form-item>
      <div class="grid-2">
        <el-form-item label="版本">
          <el-input v-model="uploadForm.version" placeholder="如 8.0.33" />
        </el-form-item>
        <el-form-item label="驱动类">
          <el-input v-model="uploadForm.driver_class" placeholder="如 com.mysql.cj.jdbc.Driver" />
        </el-form-item>
      </div>
      <el-form-item label="JAR 文件" required>
        <el-upload accept=".jar" :auto-upload="false" :limit="1" :on-change="onJarFileChange">
          <el-button plain>选择 .jar 文件</el-button>
        </el-upload>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="uploadVisible = false">取消</el-button>
      <el-button type="primary" :loading="uploading" @click="onUpload">上传</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}

.left,
.right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}
</style>
