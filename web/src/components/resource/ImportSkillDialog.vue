<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { fetchResources, importSkill } from '@/api/resource'

/** Skill 导入弹窗
 *
 * 流程:
 * 1. 父组件传入 webkitdirectory 选择后的 File[]
 * 2. 按"根目录/子目录"分组聚合,凡含 SKILL.md 的子目录视为一个 skill
 * 3. 读 SKILL.md 解析 frontmatter 取 name/description
 * 4. 拉全量已有 skill 比对 name,冲突项置灰不可勾选
 * 5. 一键导入: 调 /resources/import-skill,后端把整个子目录原样落到 ~/.aiManager/skills/{uuid}/
 */

interface ImportItem {
  /** 源子目录名(列表 key + 副信息) */
  subdir: string
  /** 解析得到的 skill 名称 */
  name: string
  /** 解析得到的描述 */
  description: string
  /** 该子目录下所有文件 */
  files: File[]
  /** 与 files 同序的相对路径(相对子目录) */
  relPaths: string[]
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
    const existing = await fetchResources({ type: 'skill', page: 1, page_size: 10000 })
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

/** 按"根目录/子目录"聚合所有文件,筛出含 SKILL.md 的子目录 */
async function scanFiles(files: File[]): Promise<Omit<ImportItem, 'selected' | 'conflict'>[]> {
  // key = 子目录名, value = { files, relPaths, skillFile }
  const groups = new Map<string, { files: File[]; relPaths: string[]; skillFile: File | null }>()

  for (const f of files) {
    const parts = f.webkitRelativePath.split('/')
    // 至少要有 根目录/子目录/文件 三段
    if (parts.length < 3) continue
    const subdir = parts[1]
    // 子目录下的相对路径(相对该 skill 子目录)
    const relPath = parts.slice(2).join('/')

    let g = groups.get(subdir)
    if (!g) {
      g = { files: [], relPaths: [], skillFile: null }
      groups.set(subdir, g)
    }
    g.files.push(f)
    g.relPaths.push(relPath)
    // 直接子文件中名为 SKILL.md 的视为 frontmatter 来源(必须 parts.length === 3)
    if (parts.length === 3 && parts[2] === 'SKILL.md') {
      g.skillFile = f
    }
  }

  const result: Omit<ImportItem, 'selected' | 'conflict'>[] = []
  for (const [subdir, g] of groups) {
    if (!g.skillFile) continue // 没 SKILL.md 的子目录跳过
    const skillText = await g.skillFile.text()
    const meta = parseFrontmatter(skillText)
    const name = (meta.name || subdir).trim()
    if (!name) continue
    result.push({
      subdir,
      name,
      description: (meta.description || '').trim(),
      files: g.files,
      relPaths: g.relPaths,
    })
  }
  return result
}

/** 极简 YAML frontmatter 解析,只支持单行 key: value */
function parseFrontmatter(text: string): { name?: string; description?: string } {
  const m = text.match(/^---\s*\r?\n([\s\S]*?)\r?\n---/)
  if (!m) return {}
  const out: Record<string, string> = {}
  for (const line of m[1].split(/\r?\n/)) {
    const km = line.match(/^(\w+):\s*(.*)$/)
    if (!km) continue
    let val = km[2].trim()
    if (
      (val.startsWith('"') && val.endsWith('"')) ||
      (val.startsWith("'") && val.endsWith("'"))
    ) {
      val = val.slice(1, -1)
    }
    out[km[1]] = val
  }
  return out
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
      await importSkill({
        name: item.name,
        description: item.description || undefined,
        files: item.files,
        relPaths: item.relPaths,
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
          Skill 导入确认
        </div>
        <div
          class="text-xs text-gray-500 dark:text-gray-400 mt-1 truncate"
          :title="rootDirName"
        >{{ rootDirName || '—' }}</div>
      </div>
    </template>

    <div v-if="loading" class="py-10 text-center text-gray-400 text-sm">扫描中...</div>
    <div v-else-if="items.length === 0" class="py-10 text-center text-gray-400 text-sm">
      未找到包含 SKILL.md 的直接子文件夹
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
          :key="item.subdir"
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
