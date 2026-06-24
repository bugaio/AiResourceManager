import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useUiStore } from '@/stores/ui'
import type { ResourceType } from '@/types/resource'

/** 资源类型集合，用于路由约束与守卫校验 */
const RESOURCE_TYPES: ResourceType[] = ['skill', 'agent', 'config', 'prompt']

/** 路由配置表
 *
 * 资源类型(skill/agent/config/prompt)各占一个顶层 URL，刷新后由路由守卫
 * 还原 uiStore.currentType，从而停留在对应模块；preset 为独立路由。
 */
const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/skill',
  },
  {
    // 4 个资源类型共用 ResourcesView，类型由 :type 段承载
    path: '/:type(skill|agent|config|prompt)',
    name: 'Resources',
    component: () => import('@/views/ResourcesView.vue'),
  },
  {
    path: '/presets',
    name: 'Presets',
    component: () => import('@/views/PresetsView.vue'),
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
  {
    // 未知路径回退到默认资源类型
    path: '/:pathMatch(.*)*',
    redirect: '/skill',
  },
]

/** 创建路由实例 */
const router = createRouter({
  history: createWebHistory(),
  routes,
})

/** 进入资源类型路由时，把类型同步到 uiStore（单一数据源） */
router.beforeEach((to) => {
  const t = to.params.type
  if (typeof t === 'string' && RESOURCE_TYPES.includes(t as ResourceType)) {
    useUiStore().setType(t as ResourceType)
  }
})

export default router
