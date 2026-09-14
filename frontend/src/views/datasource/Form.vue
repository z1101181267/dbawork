<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import ServerTunnel from './ServerTunnel.vue'
import { createDatasource, getDatasource, testAdhoc, updateDatasource } from '../../api/datasource'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  editing: { type: Object, default: null },
  dbTypes: { type: Array, default: () => [] },
  groups: { type: Array, default: () => [] }
})
const emit = defineEmits(['update:modelValue', 'saved'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const treeProps = { label: 'name', children: 'children' }
const selectableGroups = computed(() => props.groups?.[0]?.children || [])

const form = reactive({
  name: '',
  group_id: undefined,
  db_type: '',
  host: '127.0.0.1',
  port: undefined,
  db_name: '',
  username: '',
  password: '',
  extra_params: '',
  tunnel_type: 'none',
  tunnel_host: '',
  tunnel_port: undefined,
  tunnel_user: '',
  tunnel_password: '',
  tunnel_private_key: '',
  tunnel_key_passphrase: '',
  tunnel_auth_scheme: '',
  tunnel_transport: '',
  tunnel_use_ssl: false
})

const quick = reactive({ charset: '', service_name: '', sslmode: '' })
const extraRest = ref('')
const saving = ref(false)
const testing = ref(false)
const testResult = ref(null)

const QUICK_KEYS = ['charset', 'service_name', 'sslmode']

const typeMeta = computed(() => props.dbTypes.find((t) => t.key === form.db_type) || null)

function splitExtra(raw) {
  const q = { charset: '', service_name: '', sslmode: '' }
  let rest = ''
  if (!raw) return { q, rest }
  try {
    const obj = JSON.parse(raw)
    const others = {}
    for (const [k, v] of Object.entries(obj)) {
      if (QUICK_KEYS.includes(k)) q[k] = String(v)
      else others[k] = v
    }
    rest = Object.keys(others).length ? JSON.stringify(others, null, 2) : ''
  } catch {
    rest = raw
  }
  return { q, rest }
}

function mergeExtra() {
  const obj = {}
  for (const k of QUICK_KEYS) {
    if (quick[k] !== '' && quick[k] !== undefined) obj[k] = quick[k]
  }
  const rest = extraRest.value.trim()
  if (rest) {
    let parsed
    try {
      parsed = JSON.parse(rest)
    } catch {
      throw new Error('附加参数不是合法的 JSON')
    }
    if (typeof parsed !== 'object' || Array.isArray(parsed)) {
      throw new Error('附加参数必须是 JSON 对象')
    }
    Object.assign(obj, parsed)
  }
  return Object.keys(obj).length ? JSON.stringify(obj) : ''
}

function resetForm() {
  Object.assign(form, {
    name: '',
    group_id: undefined,
    db_type: '',
    host: '127.0.0.1',
    port: undefined,
    db_name: '',
    username: '',
    password: '',
    extra_params: '',
    tunnel_type: 'none',
    tunnel_host: '',
    tunnel_port: undefined,
    tunnel_user: '',
    tunnel_password: '',
    tunnel_private_key: '',
    tunnel_key_passphrase: '',
    tunnel_auth_scheme: '',
    tunnel_transport: '',
    tunnel_use_ssl: false
  })
  quick.charset = ''
  quick.service_name = ''
  quick.sslmode = ''
  extraRest.value = ''
  testResult.value = null
}

async function loadDetail() {
  const ds = await getDatasource(props.editing.id)
  Object.assign(form, {
    name: ds.name,
    group_id: ds.group_id || undefined,
    db_type: ds.db_type,
    host: ds.host,
    port: ds.port,
    db_name: ds.db_name,
    username: ds.username,
    password: '',
    extra_params: ds.extra_params || '',
    tunnel_type: ds.tunnel_type || 'none',
    tunnel_host: ds.tunnel_host || '',
    tunnel_port: ds.tunnel_port || undefined,
    tunnel_user: ds.tunnel_user || '',
    tunnel_password: '',
    tunnel_private_key: '',
    tunnel_key_passphrase: '',
    tunnel_auth_scheme: ds.tunnel_auth_scheme || '',
    tunnel_transport: ds.tunnel_transport || '',
    tunnel_use_ssl: !!ds.tunnel_use_ssl
  })
  const { q, rest } = splitExtra(ds.extra_params || '')
  quick.charset = q.charset
  quick.service_name = q.service_name
  quick.sslmode = q.sslmode
  extraRest.value = rest
  testResult.value = null
}

watch(
  () => props.modelValue,
  async (v) => {
    if (!v) return
    if (props.editing) {
      await loadDetail()
    } else {
      resetForm()
    }
  }
)

watch(
  () => form.db_type,
  (t) => {
    const meta = props.dbTypes.find((x) => x.key === t)
    if (meta && meta.default_port && !form.port) {
      form.port = meta.default_port
    }
  }
)

function buildPayload() {
  return {
    ...form,
    group_id: form.group_id || 0,
    port: form.port || 0,
    extra_params: mergeExtra()
  }
}

async function onTest() {
  let payload
  try {
    payload = buildPayload()
  } catch (e) {
    ElMessage.error(e.message)
    return
  }
  if (!form.db_type) {
    ElMessage.warning('请先选择数据库类型')
    return
  }
  testing.value = true
  testResult.value = null
  try {
    const r = await testAdhoc(payload)
    testResult.value = r
  } catch {
    /* 拦截器已提示 */
  } finally {
    testing.value = false
  }
}

async function onSave() {
  if (!form.name) return ElMessage.warning('请输入名称')
  if (!form.db_type) return ElMessage.warning('请选择数据库类型')
  let payload
  try {
    payload = buildPayload()
  } catch (e) {
    ElMessage.error(e.message)
    return
  }
  saving.value = true
  try {
    if (props.editing) {
      await updateDatasource(props.editing.id, payload)
    } else {
      await createDatasource(payload)
    }
    ElMessage.success('保存成功')
    emit('saved')
    visible.value = false
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <el-drawer v-model="visible" :title="editing ? `编辑数据源 #${editing.id}` : '新建数据源'" size="720px">
    <el-form label-position="top" class="ds-form">
      <div class="grid-2">
        <el-form-item label="名称" required>
          <el-input v-model="form.name" placeholder="如：核心交易库" />
        </el-form-item>
        <el-form-item label="分组">
          <el-tree-select
            v-model="form.group_id"
            :data="selectableGroups"
            :props="treeProps"
            node-key="id"
            check-strictly
            clearable
            :render-after-expand="false"
            placeholder="不分组"
            style="width: 100%"
          />
        </el-form-item>
      </div>

      <div class="grid-3">
        <el-form-item label="数据库类型" required>
          <el-select v-model="form.db_type" placeholder="选择类型" style="width: 100%">
            <el-option v-for="t in dbTypes" :key="t.key" :label="`${t.name_zh}（${t.key}）`" :value="t.key" />
          </el-select>
        </el-form-item>
        <el-form-item label="主机" required>
          <el-input v-model="form.host" placeholder="IP / 域名" />
        </el-form-item>
        <el-form-item label="端口">
          <el-input-number v-model="form.port" :min="0" :max="65535" controls-position="right" style="width: 100%" />
        </el-form-item>
      </div>

      <div class="grid-3">
        <el-form-item label="数据库 / 服务名">
          <el-input v-model="form.db_name" placeholder="库名 / Oracle SID·服务名 / SQLite 路径" />
        </el-form-item>
        <el-form-item label="用户名">
          <el-input v-model="form.username" placeholder="数据库账号" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :placeholder="editing ? '留空保持原值' : '登录密码'"
          />
        </el-form-item>
      </div>

      <el-divider content-position="left">扩展参数（可选）</el-divider>
      <div class="grid-3">
        <el-form-item label="字符集 charset">
          <el-input v-model="quick.charset" placeholder="utf8mb4（MySQL 系）" />
        </el-form-item>
        <el-form-item label="服务名 service_name">
          <el-input v-model="quick.service_name" placeholder="Oracle 服务名模式" />
        </el-form-item>
        <el-form-item label="ssl 模式 sslmode">
          <el-input v-model="quick.sslmode" placeholder="disable / require（PG 系）" />
        </el-form-item>
      </div>
      <el-form-item label="附加参数 JSON">
        <el-input
          v-model="extraRest"
          type="textarea"
          :rows="3"
          placeholder='如：{"connect_timeout": 10, "readonly": true}'
        />
      </el-form-item>

      <el-divider content-position="left">服务器隧道（可选）</el-divider>
      <ServerTunnel :form="form" />

      <el-alert
        v-if="testResult"
        class="test-result"
        :type="testResult.ok ? 'success' : 'error'"
        :closable="true"
        show-icon
        :title="testResult.ok ? '连接成功' : '连接失败'"
        :description="
          testResult.ok
            ? `版本：${testResult.db_version || '未知'} · 耗时：${testResult.latency_ms} ms`
            : testResult.error || '未知错误'
        "
        @close="testResult = null"
      />
    </el-form>

    <template #footer>
      <div class="footer">
        <el-button :loading="testing" @click="onTest">测试连接</el-button>
        <div>
          <el-button @click="visible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="onSave">保存</el-button>
        </div>
      </div>
    </template>
  </el-drawer>
</template>

<style scoped>
.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 16px;
}

.grid-3 {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 0 16px;
}

.test-result {
  margin-top: 8px;
}

.footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
</style>
