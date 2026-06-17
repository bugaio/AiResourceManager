<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchResources, importAgent } from '@/api/resource'
import { parseFrontmatter } from '@/utils/frontmatter'

/** SubAgent 导入弹窗
 *
 * 流程:
 * 1. 父组件传入 webkitdirectory 选择后的 File[]
 * 2. 仅扫描用户选择目录的第一级 .md 文件(parts.length === 2)
 * 3. 解析 frontmatter,有 name 才算有效;description 可空
 * 4. 与已有 agent 比对 name,冲突项置灰不可勾选
 * 5. 一键导入: 调 /resources/import-agent,后端原样落到 ~/.aiManager/agents/{uuid}.md
 */

interface ImportItem {
  /** 源文件名(不含路径,如 "foo.md"),用于列表 key */
  filename: string
  /** 解析得到的 agent 名称 */
  name: string
  /** 解析得到的描述(可空) */
  description: string
  /** 源文件 */
  file: File
  /** 是否勾选 */
  selected: boolean
  /** 是否冲突 */
  conflict: boolean
}

const props = defineProps<{
  visible: boolean
  files: File[]
  rootDirName: string
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'success'): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const loading = ref(false)
const importing = ref(false)
const items = ref<ImportItem[]>([])

watch(() => props.visible, async (val) => {
  if (val) await loadItems()
  else items.value = []
})

async function loadItems() {
  loading.value = true
  try {
    const candidates = await scanFiles(props.files)
    if (candidates.length === 0) {
      items.value = []
      return
    }
    const existing = await fetchResources({ type: 'agent', page: 1, page_size: 10000 })
    const existingNames = new Set(existing.list.map(r => r.name))
    // 同批次内重名也算冲突(后端 (type,name) 唯一)
    const seen = new Set<string>()
    const dupInBatch = new Set<string>()
    for (const c of candidates) {
      if (seen.has(c.name)) dupInBatch.add(c.name)
      seen.add(c.name)
    }
    items.value = candidates.map(c => {
      const conflict = existingNames.has(c.name) || dupInBatch.has(c.name)
      return { ...c, conflict, selected: !conflict }
    })
  } catch (e: any) {
    ElMessage.error(e?.message || '扫描失败')
    items.value = []
  } finally {
    loading.value = false
  }
}

/** 仅取用户所选目录第一级 .md 文件 */
async function scanFiles(files: File[]): Promise<Omit<ImportItem, 'selected' | 'conflict'>[]> {
  const result: Omit<ImportItem, 'selected' | 'conflict'>[] = []
  for (const f of files) {
    const parts = f.webkitRelativePath.split('/')
    // 必须刚好 2 段: 根目录 / 文件
    if (parts.length !== 2) continue
    if (!parts[1].toLowerCase().endsWith('.md')) continue
    const text = await f.text()
    const meta = parseFrontmatter(text)
    const name = (meta.name || '').trim()
    // 仅 name 解析成功才导入
    if (!name) continue
    result.push({
      filename: parts[1],
      name,
      description: (meta.description || '').trim(),
      file: f,
    })
  }
  return result
}

const selectableItems = computed(() => items.value.filter(i => !i.conflict))
const checkedItems = computed(() => items.value.filter(i => i.selected && !i.conflict))

const isAllSelected = computed(
  () => selectableItems.value.length > 0 && selectableItems.value.every(i => i.selected)
)
const isIndeterminate = computed(() => {
  const c = checkedItems.value.length
  return c > 0 && c < selectableItems.value.length
})

function handleSelectAll(val: any) {
  const checked = !!val
  for (const i of selectableItems.value) i.selected = checked
}

async function handleImport() {
  if (checkedItems.value.length === 0) return
  importing.value = true
  let ok = 0
  const fails: string[] = []
  for (const item of checkedItems.value) {
    try {
      await importAgent({
        name: item.name,
        description: item.description || undefined,
        file: item.file,
      })
      ok++
    } catch (e: any) {
      fails.push(`${item.name}: ${e?.message || '失败'}`)
    }
  }
  importing.value = false
  if (ok > 0) {
    ElMessage.success(
      `成功导入 ${ok} 个${fails.length ? `,失败 ${fails.length} 个` : ''}`
    )
    emit('success')
  }
  if (fails.length > 0 && ok === 0) {
    ElMessage.error(`导入失败: ${fails[0]}`)
  }
  if (ok > 0) dialogVisible.value = false
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    width="560px"
    :close-on-click-modal="false"
    :show-close="!importing"
  >
    <template #header>
      <div>
        <div class="text-lg font-semibold text-gray-800 dark:text-gray-100">
          SubAgent 导入确认
        </div>
        <div
          class="text-xs text-gray-500 dark:text-gray-400 mt-1 truncate"
          :title="rootDirName"
        >{{ rootDirName || '—' }}</div>
      </div>
    </template>

    <div v-if="loading" class="py-10 text-center text-gray-400 text-sm">扫描中...</div>
    <div v-else-if="items.length === 0" class="py-10 text-center text-gray-400 text-sm">
      未找到含 frontmatter name 的 .md 文件
    </div>
    <template v-else>
      <div class="flex items-center gap-3 pb-2 border-b border-gray-200 dark:border-gray-700">
        <el-checkbox
          :model-value="isAllSelected"
          :indeterminate="isIndeterminate"
          :disabled="selectableItems.length === 0"
          @change="handleSelectAll"
        >全选</el-checkbox>
        <span class="text-xs text-gray-400">
          共 {{ items.length }} 个 · 可导入 {{ selectableItems.length }} 个
        </span>
      </div>

      <div class="max-h-[360px] overflow-y-auto py-1">
        <div
          v-for="item in items"
          :key="item.filename"
          class="flex items-center justify-between gap-2 px-2 py-2 rounded hover:bg-gray-50 dark:hover:bg-gray-800/50"
          :class="item.conflict ? 'opacity-60' : ''"
        >
          <el-checkbox
            v-model="item.selected"
            :disabled="item.conflict"
            class="flex-1 min-w-0"
          >
            <span
              class="text-sm text-gray-800 dark:text-gray-100 truncate"
              :title="item.name"
            >{{ item.name }}</span>
          </el-checkbox>
          <span
            v-if="item.conflict"
            class="text-xs text-red-500 dark:text-red-400 shrink-0"
          >(检测到重复)</span>
        </div>
      </div>
    </template>

    <template #footer>
      <el-button :disabled="importing" @click="dialogVisible = false">取消</el-button>
      <el-button
        type="primary"
        :loading="importing"
        :disabled="checkedItems.length === 0"
        @click="handleImport"
      >一键导入{{ checkedItems.length > 0 ? ` (${checkedItems.length})` : '' }}</el-button>
    </template>
  </el-dialog>
</template>
