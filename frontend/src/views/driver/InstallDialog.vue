<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { installDriver } from '../../api/driver'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  row: { type: Object, default: null }
})
const emit = defineEmits(['update:modelValue', 'saved'])

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v)
})

const form = reactive({ package_name: '', version: '' })
const loading = ref(false)

watch(
  () => props.modelValue,
  (v) => {
    if (v) {
      form.package_name = props.row?.package_name || ''
      form.version = ''
    }
  }
)

async function onSubmit() {
  if (!form.package_name) return ElMessage.warning('请输入 pip 包名')
  loading.value = true
  try {
    const r = await installDriver({
      db_type: props.row?.db_type,
      driver_kind: 'python',
      package_name: form.package_name,
      version: form.version || ''
    })
    if (r.ok) {
      ElMessage.success(r.message || '安装成功')
      emit('saved')
      visible.value = false
    } else {
      ElMessage.error(r.message || '安装失败')
    }
  } catch {
    /* 拦截器已提示 */
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <el-dialog v-model="visible" :title="`安装驱动 · ${row?.db_type || ''}`" width="520px">
    <el-form label-position="top">
      <el-form-item label="pip 包名" required>
        <el-input v-model="form.package_name" placeholder="如 pymysql / psycopg2-binary" />
      </el-form-item>
      <el-form-item label="版本（可选）">
        <el-input v-model="form.version" placeholder="留空安装最新版，如 1.1.1" />
      </el-form-item>
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="安装使用当前 Python 运行时环境的 pip 执行，可能需要一些时间。"
      />
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="loading" @click="onSubmit">开始安装</el-button>
    </template>
  </el-dialog>
</template>
