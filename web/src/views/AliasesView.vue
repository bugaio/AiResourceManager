<script setup lang="ts">
import { ref, computed, onMounted, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { useAliasStore } from '@/stores/alias'
import { useDeployStore } from '@/stores/deploy'
import { useUiStore } from '@/stores/ui'
import type { PathAlias } from '@/types/alias'

/** 目录别名管理视图 */
const router = useRouter()
const aliasStore = useAliasStore()
const deployStore = useDeployStore()
const uiStore = useUiStore()

// MCP 模块的别名路径必须指向 .json 文件
const isMcp = computed(() => uiStore.currentType === 'mcp')

// 搜索关键词
const searchInput = ref('')

// 新建/编辑对话框状态
const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingAlias = ref<PathAlias | null>(null)
const formRef = ref<FormInstance>()
const submitting = ref(false)

// 表单数据
const form = reactive({ name: '', path: '' })

// 表单验证规则
const rules: FormRules = {
  name: [{ required: true, message: '请输入别名名称', trigger: 'blur' }],
  path: [{ required: true, message: '请输入路径', trigger: 'blur' }],
}

/** 按名称过滤别名列表 */
const filteredAliases = computed(() => {
  const keyword = searchInput.value.trim().toLowerCase()
  if (!keyword) return aliasStore.aliases
  return aliasStore.aliases.filter(a => a.name.toLowerCase().includes(keyword))
})

/** 返回资源页 */
function goBack() {
  router.push('/resources')
}

/** 打开新建对话框 */
function handleCreate() {
  dialogMode.value = 'create'
  editingAlias.value = null
  form.name = ''
  form.path = ''
  dialogVisible.value = true
}

/** 打开编辑对话框 */
function handleEdit(alias: PathAlias) {
  dialogMode.value = 'edit'
  editingAlias.value = alias
  form.name = alias.name
  form.path = alias.path
  dialogVisible.value = true
}

/** 提交新建/编辑表单 */
async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    // MCP 别名路径必须是 .json 文件
    if (isMcp.value && !form.path.trim().toLowerCase().endsWith('.json')) {
      ElMessage.warning('MCP 的路径必须指向 .json 文件')
      submitting.value = false
      return
    }
    if (dialogMode.value === 'create') {
      await aliasStore.createAlias({ name: form.name, path: form.path })
      ElMessage.success('别名创建成功')
    } else {
      await aliasStore.updateAlias(editingAlias.value!.id, { name: form.name, path: form.path })
      ElMessage.success('别名更新成功')
    }
    dialogVisible.value = false
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

/** 删除别名 */
async function handleDelete(alias: PathAlias) {
  // 检查该别名路径下是否有部署记录
  await deployStore.fetchTargets()
  const relatedTarget = deployStore.targets.find(t => t.target_path === alias.path)
  const hasDeployments = relatedTarget && relatedTarget.deployments.length > 0

  const message = hasDeployments
    ? `确定要删除别名「${alias.name}」吗？该路径下有 ${relatedTarget.deployments.length} 条部署记录，将一并撤销。`
    : `确定要删除别名「${alias.name}」吗？`

  try {
    await ElMessageBox.confirm(
      message,
      '确认删除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    // 先撤销关联部署
    if (hasDeployments) {
      for (const d of relatedTarget.deployments) {
        await deployStore.undeploy(d.id)
      }
    }
    await aliasStore.deleteAlias(alias.id)
    ElMessage.success('删除成功')
  } catch {
    // 用户取消
  }
}

/** 格式化时间 */
function formatTime(t: string) {
  if (!t) return ''
  return new Date(t).toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

onMounted(() => {
  aliasStore.fetchAliases()
})
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
      <h1 class="ml-4 text-lg font-semibold text-gray-800 dark:text-gray-100">目录别名管理</h1>
    </div>

    <!-- 工具栏：搜索 + 新增 -->
    <div class="flex items-center justify-between gap-4 px-6 pt-5 pb-3">
      <el-input
        v-model="searchInput"
        placeholder="搜索别名..."
        clearable
        class="max-w-xs"
      />
      <el-button type="primary" @click="handleCreate">新增别名</el-button>
    </div>

    <!-- 表格内容区 -->
    <div class="flex-1 overflow-auto px-6 pb-6">
      <el-table
        :data="filteredAliases"
        v-loading="aliasStore.loading"
        stripe
        class="w-full dark:bg-gray-800"
      >
        <el-table-column prop="name" label="别名名称" min-width="160" />
        <el-table-column prop="path" label="路径" min-width="240" />
        <el-table-column label="创建时间" min-width="180">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" text size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button type="danger" text size="small" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogMode === 'create' ? '新增别名' : '编辑别名'"
      width="440px"
      :close-on-click-modal="false"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
      >
        <el-form-item label="别名名称" prop="name">
          <el-input v-model="form.name" placeholder="输入别名名称" maxlength="50" />
        </el-form-item>
        <el-form-item label="路径" prop="path">
          <el-input v-model="form.path" :placeholder="isMcp ? '指向 .json 文件，如 ~/Library/.../claude_desktop_config.json' : '支持 ~ 表示主目录'" maxlength="500" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ dialogMode === 'create' ? '创建' : '保存' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>
