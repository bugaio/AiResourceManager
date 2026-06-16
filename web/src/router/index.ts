import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

/** 路由配置表 */
const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/resources',
  },
  {
    path: '/resources',
    name: 'Resources',
    component: () => import('@/views/ResourcesView.vue'),
  },
  {
    path: '/aliases',
    name: 'Aliases',
    component: () => import('@/views/AliasesView.vue'),
  },
  {
    path: '/data',
    name: 'Data',
    component: () => import('@/views/DataView.vue'),
  },
]

/** 创建路由实例 */
const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
