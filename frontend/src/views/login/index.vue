<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Lock, User } from '@element-plus/icons-vue'
import { login } from '../../api/auth'

const router = useRouter()
const loading = ref(false)
const form = reactive({ username: 'admin', password: '' })

async function onSubmit() {
  if (!form.username || !form.password) {
    ElMessage.warning('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    const res = await login({ username: form.username, password: form.password })
    localStorage.setItem('token', res.token)
    ElMessage.success('登录成功')
    router.push('/datasources')
  } catch {
    // 错误提示已由拦截器统一处理
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <el-card class="login-card" shadow="always">
      <div class="brand">
        <span class="brand-mark">DB</span>
        <span class="brand-name">DBAWORK</span>
      </div>
      <div class="subtitle">数据库综合管理平台 · 登录</div>
      <el-form label-position="top" @submit.prevent>
        <el-form-item label="用户名">
          <el-input v-model="form.username" :prefix-icon="User" placeholder="admin" @keyup.enter="onSubmit" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input
            v-model="form.password"
            :prefix-icon="Lock"
            type="password"
            show-password
            placeholder="默认 admin / admin"
            @keyup.enter="onSubmit"
          />
        </el-form-item>
        <el-button type="primary" class="submit" :loading="loading" @click="onSubmit">登 录</el-button>
      </el-form>
      <div class="tip">默认账号 admin / admin（可在 Go 后端环境变量中修改）</div>
    </el-card>
  </div>
</template>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: radial-gradient(1200px 600px at 20% 0%, #23324a 0%, #131c2b 55%, #0e1522 100%);
}

.login-card {
  width: 380px;
  padding: 8px 12px 4px;
  border-radius: 12px;
}

.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 6px;
  justify-content: center;
}

.brand-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  background: #2f7bd9;
  color: #fff;
  font-weight: 700;
}

.brand-name {
  font-size: 20px;
  font-weight: 700;
  letter-spacing: 2px;
}

.subtitle {
  text-align: center;
  color: #909399;
  font-size: 13px;
  margin-bottom: 18px;
}

.submit {
  width: 100%;
}

.tip {
  margin-top: 12px;
  font-size: 12px;
  color: #a8abb2;
  text-align: center;
  padding-bottom: 6px;
}
</style>
