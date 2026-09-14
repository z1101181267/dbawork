import { createRouter, createWebHistory } from 'vue-router'
import AppLayout from '../layout/AppLayout.vue'

const routes = [
  {
    path: '/',
    component: AppLayout,
    redirect: '/datasources',
    children: [
      {
        path: 'datasources',
        name: 'datasources',
        component: () => import('../views/datasource/index.vue'),
        meta: { title: '数据源配置' }
      },
      {
        path: 'drivers',
        name: 'drivers',
        component: () => import('../views/driver/index.vue'),
        meta: { title: '驱动管理' }
      }
    ]
  },
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/login/index.vue'),
    meta: { title: '登录' }
  },
  { path: '/:pathMatch(.*)*', redirect: '/datasources' }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.afterEach((to) => {
  document.title = to.meta?.title ? `${to.meta.title} · DBAWORK` : 'DBAWORK 数据库管理平台'
})

export default router
