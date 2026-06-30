<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { exportData, importData } from '@/api/data'
import { ApiError } from '@/api/request'
import { useUiStore } from '@/stores/ui'

/** 数据导入导出视图 */
const router = useRouter()
const uiStore = useUiStore()

// 导出状态
const exportPath = ref('')
const exporting = ref(false)
const exportResult = ref<{
  resource_count: number
  group_count: number
  preset_count: number
  file_count: number
  total_size: number
} | null>(null)

// 导入状态
const importPath = ref('')
const importStrategy = ref<'overwrite' | 'skip' | 'keep_both'>('skip')
const importing = ref(false)
const importResult = ref<{
  added: number
  overwritten: number
  skipped: number
  renamed: number
} | null>(null)

/** 返回来源资源模块（currentType 仍保留进入前的类型） */
function goBack() {
  router.push(`/${uiStore.currentType}`)
}

/** 格式化文件大小 */
function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
}

/** 执行导出
 *  目标目录非空(忽略隐藏文件)时后端返回 5004，弹窗确认后带 clear 重试 */
async function handleExport() {
  if (!exportPath.value.trim()) {
    ElMessage.warning('请输入导出目录路径')
    return
  }
  const path = exportPath.value.trim()
  exporting.value = true
  exportResult.value = null
  try {
    await runExport(path, false)
  } catch (e: any) {
    if (e instanceof ApiError && e.code === 5004) {
      // 目标目录不为空 → 让用户确认是否清除(隐藏文件如 .git 保留)
      try {
        await ElMessageBox.confirm(
          '目标目录不为空，是否清除其中的文件后再导出？（隐藏文件如 .git 会保留）',
          '目标目录不为空',
          { confirmButtonText: '清除并导出', cancelButtonText: '取消', type: 'warning' }
        )
      } catch {
        // 用户取消，什么都不干
        exporting.value = false
        return
      }
      try {
        await runExport(path, true)
      } catch (e2: any) {
        ElMessage.error(e2?.message || '导出失败')
      } finally {
        exporting.value = false
      }
      return
    }
    ElMessage.error(e?.message || '导出失败')
  } finally {
    exporting.value = false
  }
}

/** 调用导出接口并写入结果 */
async function runExport(path: string, clear: boolean) {
  const res = await exportData(path, clear)
  exportResult.value = res
  ElMessage.success('导出完成')
}

/** 执行导入 */
async function handleImport() {
  if (!importPath.value.trim()) {
    ElMessage.warning('请输入来源目录路径')
    return
  }
  importing.value = true
  importResult.value = null
  try {
    const res = await importData(importPath.value.trim(), importStrategy.value)
    importResult.value = res
    ElMessage.success('导入完成')
  } catch (e: any) {
    ElMessage.error(e?.message || '导入失败')
  } finally {
    importing.value = false
  }
}
</script>

<template>
  <div class="h-screen flex flex-col bg-white dark:bg-gray-900">
    <!-- 顶部栏 -->
    <div class="h-14 min-h-[56px] flex items-center px-4 border-b border-gray-200 dark:border-gray-700">
      <button
        class="flex items-center gap-1 text-sm text-gray-600 dark:text-gray-300 hover:text-blue-500 transition-colors"
        @click="goBack"
      >
        <el-icon><ArrowLeft /></el-icon>
        <span>返回</span>
      </button>
      <h1 class="ml-4 text-lg font-semibold text-gray-800 dark:text-gray-100">数据导入导出</h1>
    </div>

    <!-- 内容区 -->
    <div class="flex-1 overflow-auto p-6">
      <div class="max-w-2xl mx-auto mt-8 flex flex-col gap-6">

        <!-- 导出卡片 -->
        <div class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-6 shadow-sm">
          <h2 class="text-base font-semibold text-gray-800 dark:text-gray-100 mb-1">数据导出</h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">导出资源实体、分组与 preset 关联到指定目录(可对其 git init 跨机器同步，不含部署信息)</p>
          <div class="flex items-center gap-3">
            <el-input
              v-model="exportPath"
              placeholder="目标路径，如 ~/export-data"
              clearable
              class="flex-1"
            />
            <el-button type="primary" :loading="exporting" @click="handleExport">导出</el-button>
          </div>
          <p v-if="exportResult" class="mt-3 text-sm text-green-600 dark:text-green-400">
            导出完成：{{ exportResult.resource_count }} 个资源、{{ exportResult.group_count }} 个分组、{{ exportResult.preset_count }} 个 preset，共 {{ exportResult.file_count }} 个文件（{{ formatSize(exportResult.total_size) }}）
          </p>
        </div>

        <!-- 导入卡片 -->
        <div class="rounded-xl border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 p-6 shadow-sm">
          <h2 class="text-base font-semibold text-gray-800 dark:text-gray-100 mb-1">数据导入</h2>
          <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">从指定目录导入数据到当前仓库</p>
          <el-input
            v-model="importPath"
            placeholder="来源路径，如 ~/export-data"
            clearable
            class="mb-4"
          />
          <div class="mb-4">
            <span class="text-sm text-gray-600 dark:text-gray-300 mr-3">冲突策略：</span>
            <el-radio-group v-model="importStrategy">
              <el-radio value="overwrite">覆盖已有</el-radio>
              <el-radio value="skip">跳过重复</el-radio>
              <el-radio value="keep_both">两者保留</el-radio>
            </el-radio-group>
          </div>
          <el-button type="primary" :loading="importing" @click="handleImport">导入</el-button>
          <p v-if="importResult" class="mt-3 text-sm text-green-600 dark:text-green-400">
            导入完成：新增 {{ importResult.added }}，覆盖 {{ importResult.overwritten }}，跳过 {{ importResult.skipped }}，副本 {{ importResult.renamed }}
          </p>
        </div>

      </div>
    </div>
  </div>
</template>
