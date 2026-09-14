<script setup>
import { computed, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Download, Edit, Plus, RefreshLeft, Search, UploadFilled } from '@element-plus/icons-vue'
import DbTypeTag from '../../components/DbTypeTag.vue'
import StatusBadge from '../../components/StatusBadge.vue'
import FormDrawer from './Form.vue'
import ImportCSV from './ImportCSV.vue'
import { listDbTypes } from '../../api/driver'
import { exportCsv, listDatasources, removeDatasource, testDatasource } from '../../api/datasource'
import { createGroup, listGroups, removeGroup, updateGroup } from '../../api/group'

const treeRef = ref()
const treeProps = { label: 'name', children: 'children' }

const rawTree = ref([])
const treeData = computed(() => [{ id: 0, name: '全部', children: rawTree.value }])
const activeGroupId = ref(0)

const filters = reactive({ db_type: '', status: '', keyword: '' })
const list = ref([])
const loading = ref(false)
const selection = ref([])
const testingId = ref(null)

const dbTypes = ref([])
const formVisible = ref(false)
const editingRow = ref(null)
const importVisible = ref(false)

const typeLabelMap = computed(() => {
  const m = {}
  for (const t of dbTypes.value) m[t.key] = t.name_zh
  return m
})
const typeLabel = (key) => typeLabelMap.value[key] || key

function fmtTime(s) {
  if (!s) return '—'
  return String(s).replace('T', ' ').slice(0, 16)
}

async function loadGroups() {
  try {
    rawTree.value = await listGroups()
  } catch {
    rawTree.value = []
  }
  await nextTick()
  try {
    treeRef.value?.setCurrentKey(activeGroupId.value)
  } catch {
    /* ignore */
  }
}

async function loadList() {
  loading.value = true
  try {
    list.value = await listDatasources({
      group_id: activeGroupId.value || undefined,
      db_type: filters.db_type || undefined,
      status: filters.status === '' ? undefined : filters.status,
      keyword: filters.keyword || undefined
    })
  } finally {
    loading.value = false
  }
}

function onGroupClick(data) {
  activeGroupId.value = data.id
  loadList()
}

function resetFilters() {
  filters.db_type = ''
  filters.status = ''
  filters.keyword = ''
  loadList()
}

async function onCreateRootGroup() {
  const { value } = await ElMessageBox.prompt('请输入分组名称', '新建分组', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '名称不能为空'
  })
  await createGroup({ name: value, parent_id: 0 })
  ElMessage.success('分组已创建')
  loadGroups()
}

async function onCreateChildGroup(data) {
  const { value } = await ElMessageBox.prompt(`在「${data.name}」下新建子分组`, '新建子分组', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputPattern: /\S+/,
    inputErrorMessage: '名称不能为空'
  })
  await createGroup({ name: value, parent_id: data.id })
  ElMessage.success('子分组已创建')
  loadGroups()
}

async function onRenameGroup(data) {
  const { value } = await ElMessageBox.prompt('请输入新名称', `重命名「${data.name}」`, {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    inputValue: data.name,
    inputPattern: /\S+/,
    inputErrorMessage: '名称不能为空'
  })
  await updateGroup(data.id, {
    name: value,
    parent_id: data.parent_id || 0,
    sort_order: data.sort_order || 0,
    description: data.description || ''
  })
  ElMessage.success('已重命名')
  loadGroups()
}

async function onDeleteGroup(data) {
  await ElMessageBox.confirm(`确定删除分组「${data.name}」吗？（需先清空其中的数据源）`, '删除确认', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await removeGroup(data.id)
  ElMessage.success('分组已删除')
  if (activeGroupId.value === data.id) activeGroupId.value = 0
  loadGroups()
  loadList()
}

function openForm(row = null) {
  editingRow.value = row
  formVisible.value = true
}

async function onTest(row) {
  testingId.value = row.id
  try {
    const r = await testDatasource(row.id)
    if (r.ok) {
      ElMessage.success(`连接成功：${r.db_version || 'OK'}（${r.latency_ms} ms）`)
    } else {
      ElMessage.error(`连接失败：${r.error || '未知错误'}`)
    }
    await loadList()
  } finally {
    testingId.value = null
  }
}

async function onDelete(row) {
  await ElMessageBox.confirm(`确定删除数据源「${row.name}」吗？`, '删除确认', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await removeDatasource(row.id)
  ElMessage.success('已删除')
  loadList()
}

async function onBatchDelete() {
  if (!selection.value.length) return
  await ElMessageBox.confirm(`确定删除选中的 ${selection.value.length} 个数据源吗？`, '批量删除', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  for (const row of selection.value) {
    try {
      await removeDatasource(row.id)
    } catch {
      /* 单个失败继续 */
    }
  }
  ElMessage.success('批量删除完成')
  loadList()
}

async function onExport() {
  const blob = await exportCsv()
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = 'datasources.csv'
  a.click()
  URL.revokeObjectURL(a.href)
}

onMounted(async () => {
  dbTypes.value = await listDbTypes().catch(() => [])
  await loadGroups()
  await loadList()
})
</script>

<template>
  <div class="ds-page">
    <el-card class="side" shadow="never">
      <div class="side-head">
        <span class="side-title">数据源分组</span>
        <el-button size="small" type="primary" plain :icon="Plus" @click="onCreateRootGroup">新建分组</el-button>
      </div>
      <el-tree
        ref="treeRef"
        :data="treeData"
        node-key="id"
        :props="treeProps"
        default-expand-all
        highlight-current
        :expand-on-click-node="false"
        @node-click="onGroupClick"
      >
        <template #default="{ data }">
          <span class="tree-node">
            <span class="tree-label">{{ data.name }}</span>
            <span v-if="data.id !== 0" class="tree-actions">
              <el-icon title="新建子分组" @click.stop="onCreateChildGroup(data)"><Plus /></el-icon>
              <el-icon title="重命名" @click.stop="onRenameGroup(data)"><Edit /></el-icon>
              <el-icon title="删除" @click.stop="onDeleteGroup(data)"><Delete /></el-icon>
            </span>
          </span>
        </template>
      </el-tree>
    </el-card>

    <div class="content">
      <el-card shadow="never" class="filter-card">
        <div class="filters">
          <el-select v-model="filters.db_type" placeholder="数据库类型" clearable style="width: 170px">
            <el-option v-for="t in dbTypes" :key="t.key" :label="t.name_zh" :value="t.key" />
          </el-select>
          <el-select v-model="filters.status" placeholder="状态" clearable style="width: 120px">
            <el-option label="未测" :value="0" />
            <el-option label="正常" :value="1" />
            <el-option label="异常" :value="2" />
          </el-select>
          <el-input
            v-model="filters.keyword"
            placeholder="名称 / 主机"
            clearable
            style="width: 210px"
            @keyup.enter="loadList"
          />
          <el-button :icon="Search" @click="loadList">查询</el-button>
          <el-button :icon="RefreshLeft" @click="resetFilters">重置</el-button>
        </div>
        <div class="toolbar">
          <el-button type="primary" :icon="Plus" @click="openForm()">新建数据源</el-button>
          <el-button :icon="UploadFilled" @click="importVisible = true">批量导入</el-button>
          <el-button :icon="Download" @click="onExport">导出 CSV</el-button>
          <el-button type="danger" plain :icon="Delete" :disabled="!selection.length" @click="onBatchDelete">
            批量删除{{ selection.length ? `（${selection.length}）` : '' }}
          </el-button>
        </div>
      </el-card>

      <el-card shadow="never" class="table-card">
        <el-table
          v-loading="loading"
          :data="list"
          stripe
          empty-text="暂无数据源，点击「新建数据源」开始"
          @selection-change="(v) => (selection = v)"
        >
          <el-table-column type="selection" width="42" />
          <el-table-column prop="id" label="ID" width="46" />
          <el-table-column prop="name" label="名称" min-width="120" show-overflow-tooltip />
          <el-table-column label="类型" width="110">
            <template #default="{ row }">
              <DbTypeTag :value="row.db_type" :label="typeLabel(row.db_type)" />
            </template>
          </el-table-column>
          <el-table-column prop="host" label="主机" min-width="90" show-overflow-tooltip />
          <el-table-column prop="port" label="端口" width="62" />
          <el-table-column prop="db_name" label="库名" min-width="90" show-overflow-tooltip />
          <el-table-column label="隧道" width="60">
            <template #default="{ row }">
              <span v-if="row.tunnel_type === 'ssh'">SSH</span>
              <span v-else-if="row.tunnel_type === 'winrm'">WinRM</span>
              <span v-else class="muted">—</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="68">
            <template #default="{ row }">
              <StatusBadge :status="row.status" :last-error="row.last_error" />
            </template>
          </el-table-column>
          <el-table-column label="最近测试" width="150">
            <template #default="{ row }">{{ fmtTime(row.last_test_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="175" fixed="right">
            <template #default="{ row }">
              <span class="op-btns">
                <el-button size="small" text type="primary" :loading="testingId === row.id" @click="onTest(row)">
                  测试
                </el-button>
                <el-button size="small" text @click="openForm(row)">编辑</el-button>
                <el-button size="small" text type="danger" @click="onDelete(row)">删除</el-button>
              </span>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </div>

    <FormDrawer
      v-model="formVisible"
      :editing="editingRow"
      :db-types="dbTypes"
      :groups="treeData"
      @saved="loadList"
    />
    <ImportCSV v-model="importVisible" @imported="loadList" />
  </div>
</template>

<style scoped>
.ds-page {
  display: flex;
  gap: 16px;
  align-items: flex-start;
}

.side {
  width: 268px;
  flex: none;
}

.side-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.side-title {
  font-weight: 600;
}

.tree-node {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding-right: 6px;
}

.tree-actions {
  display: none;
  gap: 8px;
  color: #909399;
}

.tree-node:hover .tree-actions {
  display: inline-flex;
}

.tree-actions .el-icon:hover {
  color: #2f7bd9;
}

.content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.filters {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  align-items: center;
}

.toolbar {
  display: flex;
  gap: 10px;
  margin-top: 12px;
  border-top: 1px dashed #e4e7ed;
  padding-top: 12px;
}

.muted {
  color: #c0c4cc;
}

.op-btns :deep(.el-button) {
  padding: 4px 6px;
}

.op-btns :deep(.el-button + .el-button) {
  margin-left: 6px;
}
</style>
