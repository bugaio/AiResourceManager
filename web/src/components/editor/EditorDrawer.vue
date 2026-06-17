<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import { ElMessageBox, ElMessage } from 'element-plus'
import MonacoEditor from './MonacoEditor.vue'
import SyncDeployDialog from '@/components/deploy/SyncDeployDialog.vue'
import { useUiStore } from '@/stores/ui'
import { getResource, getContent, updateContent } from '@/api/resource'
import { getResourceDeployTargets } from '@/api/deploy'
import type { Resource } from '@/types/resource'

/** 编辑器抽屉组件 - 右侧滑出，支持拖拽调宽 */
const props = defineProps<{
  visible: boolean
  resourceId: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'saved'): void
}>()

const uiStore = useUiStore()

// 编辑器内容
const content = ref('')
// 原始内容（用于判断dirty）
const originalContent = ref('')
// 资源详情
const resource = ref<Resource | null>(null)
// 加载状态
const loading = ref(false)
// 保存中状态
const saving = ref(false)
// 同步弹窗状态
const syncDialogVisible = ref(false)
// Monaco 编辑器组件引用（用于格式化）
const monacoRef = ref<InstanceType<typeof MonacoEditor>>()
// 是否为 Config 类型
const isConfig = computed(() => resource.value?.type === 'config')
// 抽屉宽度（百分比）
const drawerWidth = ref(70)
// 是否正在拖拽
const dragging = ref(false)

/** 是否有未保存变更 */
const dirty = computed(() => content.value !== originalContent.value)

/** Monaco语言: skill/agent→markdown; config→按文件后缀选 json/yaml/toml */
const editorLanguage = computed(() => {
  if (resource.value?.type === 'config') {
    const path = (resource.value.path || '').toLowerCase()
    if (path.endsWith('.yaml') || path.endsWith('.yml')) return 'yaml'
    if (path.endsWith('.toml')) return 'toml'
    return 'json' // .json / .jsonc / 其它都走 json(jsonc 也支持注释)
  }
  return 'markdown'
})

/** Monaco主题：跟随全局暗色模式 */
const editorTheme = computed<'vs' | 'vs-dark'>(() => {
  return uiStore.theme === 'dark' ? 'vs-dark' : 'vs'
})

/** 打开时加载资源数据和内容 */
watch(() => props.visible, async (val) => {
  if (val && props.resourceId) {
    loading.value = true
    try {
      const [res, cnt] = await Promise.all([
        getResource(props.resourceId),
        getContent(props.resourceId),
      ])
      resource.value = res
      content.value = cnt
      originalContent.value = content.value
    } catch (e: any) {
      ElMessage.error(e.message || '加载资源失败')
    } finally {
      loading.value = false
    }
  }
})

/** 保存内容 */
async function handleSave() {
  if (!props.resourceId || !dirty.value) return
  saving.value = true
  try {
    await updateContent(props.resourceId, content.value)
    originalContent.value = content.value
    ElMessage.success('保存成功')
    emit('saved')

    // Config 类型：检查是否有已部署路径，有则询问同步
    if (resource.value?.type === 'config') {
      const targets = await getResourceDeployTargets(props.resourceId)
      if (targets.length === 0) {
        // 无关联部署，直接关闭
        emit('update:visible', false)
      } else {
        try {
          await ElMessageBox.confirm(
            '是否同步更新已部署的目标路径？',
            '同步部署',
            { confirmButtonText: '同步', cancelButtonText: '否', type: 'info' }
          )
          syncDialogVisible.value = true
        } catch {
          emit('update:visible', false)
        }
      }
    }
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

/** 格式化 JSON/JSONC 内容 */
function handleFormat() {
  monacoRef.value?.format()
}

/** 同步完成后关闭编辑器 */
function handleSynced() {
  emit('update:visible', false)
}

/** 关闭抽屉（检查dirty状态） */
async function handleClose() {
  if (dirty.value) {
    try {
      await ElMessageBox.confirm('内容未保存，确定关闭？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      })
    } catch {
      return // 用户取消
    }
  }
  emit('update:visible', false)
}

/** 键盘快捷键：Ctrl+S / Cmd+S 保存 */
function handleKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault()
    handleSave()
  }
}

// 拖拽调整宽度
function startDrag(e: MouseEvent) {
  e.preventDefault()
  dragging.value = true
  const startX = e.clientX
  const startWidth = drawerWidth.value

  function onMove(ev: MouseEvent) {
    // 向左拖拽增加宽度
    const diff = startX - ev.clientX
    const vw = window.innerWidth
    const newWidth = startWidth + (diff / vw) * 100
    drawerWidth.value = Math.max(40, Math.min(90, newWidth))
  }

  function onUp() {
    dragging.value = false
    document.removeEventListener('mousemove', onMove)
    document.removeEventListener('mouseup', onUp)
  }

  document.addEventListener('mousemove', onMove)
  document.addEventListener('mouseup', onUp)
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <!-- 遮罩层 -->
  <Teleport to="body">
    <Transition name="drawer-fade">
      <div
        v-if="visible"
        class="fixed inset-0 z-[2000] flex justify-end"
      >
        <!-- 背景遮罩 -->
        <div class="absolute inset-0 bg-black/30" @click="handleClose" />

        <!-- 抽屉面板 -->
        <div
          class="relative flex h-full bg-white dark:bg-gray-900 shadow-2xl"
          :style="{ width: drawerWidth + 'vw' }"
        >
          <!-- 左侧拖拽边 -->
          <div
            class="absolute left-0 top-0 bottom-0 w-1 cursor-col-resize hover:bg-blue-400/50 z-10"
            :class="{ 'bg-blue-400/50': dragging }"
            @mousedown="startDrag"
          />

          <!-- 内容区 -->
          <div class="flex flex-col w-full h-full overflow-hidden">
            <!-- 头部 -->
            <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700 shrink-0">
              <div class="flex items-center gap-3 min-w-0">
                <h3 class="text-base font-semibold text-gray-800 dark:text-gray-100 truncate">
                  {{ resource?.name || '加载中...' }}
                </h3>
                <span
                  v-if="dirty"
                  class="text-xs px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
                >
                  未保存
                </span>
              </div>
              <div class="flex items-center gap-2">
                <el-button
                  v-if="isConfig"
                  size="small"
                  @click="handleFormat"
                >
                  格式化
                </el-button>
                <el-button
                  type="primary"
                  size="small"
                  :loading="saving"
                  :disabled="!dirty"
                  @click="handleSave"
                >
                  保存
                </el-button>
                <el-button size="small" @click="handleClose">关闭</el-button>
              </div>
            </div>

            <!-- 编辑器区域 -->
            <div class="flex-1 min-h-0">
              <div v-if="loading" class="flex items-center justify-center h-full text-gray-400">
                加载中...
              </div>
              <MonacoEditor
                v-else
                ref="monacoRef"
                v-model="content"
                :language="editorLanguage"
                :theme="editorTheme"
              />
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>

  <!-- Config 同步部署弹窗 -->
  <SyncDeployDialog
    v-model:visible="syncDialogVisible"
    :resource-id="resourceId"
    @synced="handleSynced"
  />
</template>

<style scoped>
.drawer-fade-enter-active,
.drawer-fade-leave-active {
  transition: opacity 0.2s ease;
}
.drawer-fade-enter-active > div:last-child,
.drawer-fade-leave-active > div:last-child {
  transition: transform 0.25s ease;
}
.drawer-fade-enter-from,
.drawer-fade-leave-to {
  opacity: 0;
}
.drawer-fade-enter-from > div:last-child,
.drawer-fade-leave-to > div:last-child {
  transform: translateX(100%);
}
</style>
