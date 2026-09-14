<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Coin, Cpu } from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()

const activeMenu = computed(() => route.path)
const token = computed(() => localStorage.getItem('token') || '')

function logout() {
  localStorage.removeItem('token')
  router.push('/login')
}
</script>

<template>
  <el-container class="layout">
    <el-aside width="220px" class="aside">
      <div class="logo">
        <span class="logo-mark">DB</span>
        <span class="logo-text">DBAWORK</span>
      </div>
      <el-menu :default-active="activeMenu" router class="menu">
        <el-menu-item index="/datasources">
          <el-icon><Coin /></el-icon>
          <span>数据源配置</span>
        </el-menu-item>
        <el-menu-item index="/drivers">
          <el-icon><Cpu /></el-icon>
          <span>驱动管理</span>
        </el-menu-item>
      </el-menu>
      <div class="aside-foot">数据库综合管理平台</div>
    </el-aside>

    <el-container>
      <el-header class="header">
        <div class="header-title">DBAWORK 数据库管理平台</div>
        <div class="header-right">
          <template v-if="token">
            <span class="user-hint">已登录</span>
            <el-button link type="primary" @click="logout">退出</el-button>
          </template>
          <template v-else>
            <span class="user-hint">未登录</span>
            <el-button link type="primary" @click="router.push('/login')">去登录</el-button>
          </template>
        </div>
      </el-header>

      <el-main class="main">
        <router-view />
      </el-main>
    </el-container>
  </el-container>
</template>

<style scoped>
.layout {
  height: 100vh;
}

.aside {
  display: flex;
  flex-direction: column;
  background: #1d2939;
  color: #d0d5dd;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 18px 20px 14px;
}

.logo-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 8px;
  background: #2f7bd9;
  color: #fff;
  font-weight: 700;
  font-size: 14px;
  letter-spacing: 0.5px;
}

.logo-text {
  font-size: 17px;
  font-weight: 600;
  color: #f2f4f7;
  letter-spacing: 1px;
}

.menu {
  flex: 1;
  border-right: none;
  background: transparent;
  --el-menu-text-color: #c3cad4;
  --el-menu-hover-bg-color: rgba(255, 255, 255, 0.06);
  --el-menu-active-color: #7fb3ea;
}

.menu :deep(.el-menu-item.is-active) {
  background: rgba(47, 123, 217, 0.18);
  border-right: 3px solid #2f7bd9;
}

.aside-foot {
  padding: 14px 20px;
  font-size: 12px;
  color: #98a2b3;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}

.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
}

.header-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.user-hint {
  font-size: 13px;
  color: #909399;
}

.main {
  background: #f5f7fa;
  padding: 16px;
}
</style>
