<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useGroupStore } from '@/stores/group'
import { renderMarkdown, renderMarkdownWithLines } from '@/utils/markdown'

/** 分组说明卡片（可折叠 + Markdown）
 *
 * 进入具体分组时显示。默认折叠，仅露出标题行「(色点)分组名 分组说明」。
 * 展开后显示说明（Markdown 渲染），可就地编辑。编辑态为「左编辑 / 右预览」
 * 分栏，两侧按内容（源码行号↔预览块位置）滚动同步。
 * 说明为空时显示占位文案「这里什么都没有」。「全部」等无对应分组时不渲染。
 */
const props = defineProps<{ groupId: string }>()

const groupStore = useGroupStore()

/** 当前分组对象（'0'/未匹配时为空） */
const group = computed(() =>
  props.groupId !== '0' ? groupStore.groups.find((g) => g.id === props.groupId) : undefined,
)

const expanded = ref(false)
const editing = ref(false)
const draft = ref('')
const saving = ref(false)

/** 折叠时标题行显示的单行纯文本摘要：剥离 Markdown 语法、压平换行、截断 */
const plainSummary = computed(() => {
  const raw = group.value?.description || ''
  if (!raw.trim()) return ''
  const text = raw
    .replace(/```[\s\S]*?```/g, ' ')      // 代码块
    .replace(/`([^`]*)`/g, '$1')           // 行内代码
    .replace(/!\[[^\]]*\]\([^)]*\)/g, ' ')  // 图片
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1') // 链接取文字
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')     // 标题符号
    .replace(/[*_~>#-]/g, '')               // 其余强调/引用/列表符号
    .replace(/\s+/g, ' ')                   // 压平空白
    .trim()
  const max = 40
  return text.length > max ? text.slice(0, max) + '…' : text
})

/** 展示态：净化后的 HTML */
const renderedHtml = computed(() => renderMarkdown(group.value?.description || ''))
/** 编辑态预览：带源码行号锚点的 HTML（用于滚动同步） */
const previewHtml = computed(() => renderMarkdownWithLines(draft.value))

// 滚动同步用的 DOM 引用
const editorRef = ref<HTMLTextAreaElement | null>(null)
const previewRef = ref<HTMLElement | null>(null)
/** 防止程序化滚动互相触发形成死循环 */
let isSyncing = false

/** 切换分组时收起并退出编辑态，避免草稿/展开态串到别的分组 */
watch(
  () => props.groupId,
  () => {
    expanded.value = false
    editing.value = false
  },
)

function toggle() {
  if (editing.value) return // 编辑中不因点标题而收起
  expanded.value = !expanded.value
}

function startEdit() {
  draft.value = group.value?.description || ''
  editing.value = true
  expanded.value = true
}

function cancelEdit() {
  editing.value = false
}

async function save() {
  if (!group.value) return
  saving.value = true
  try {
    await groupStore.updateGroup(group.value.id, { description: draft.value.trim() })
    ElMessage.success('说明已保存')
    editing.value = false
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

/** 读取 textarea 行高（px），异常时回退 20 */
function lineHeight(): number {
  const el = editorRef.value
  if (!el) return 20
  const lh = parseFloat(getComputedStyle(el).lineHeight)
  return Number.isFinite(lh) && lh > 0 ? lh : 20
}

/** 收集预览区各块的 {源码行号, 顶部像素}，按行号升序 */
function previewBlocks(): { line: number; top: number }[] {
  const box = previewRef.value
  if (!box) return []
  const nodes = box.querySelectorAll<HTMLElement>('.md-block[data-src-line]')
  const arr: { line: number; top: number }[] = []
  nodes.forEach((n) => {
    arr.push({ line: Number(n.dataset.srcLine || 0), top: n.offsetTop })
  })
  arr.sort((a, b) => a.line - b.line)
  return arr
}

/** 在有序数组里按 key 取值找到左右相邻两点做线性插值 */
function interpolate(
  points: { line: number; top: number }[],
  key: number,
  from: 'line' | 'top',
  to: 'line' | 'top',
): number {
  if (points.length === 0) return 0
  if (key <= points[0][from]) return points[0][to]
  const last = points[points.length - 1]
  if (key >= last[from]) return last[to]
  for (let i = 0; i < points.length - 1; i++) {
    const a = points[i]
    const b = points[i + 1]
    if (key >= a[from] && key <= b[from]) {
      const span = b[from] - a[from]
      const ratio = span === 0 ? 0 : (key - a[from]) / span
      return a[to] + ratio * (b[to] - a[to])
    }
  }
  return last[to]
}

/** 编辑区滚动 → 同步预览区 */
function onEditorScroll() {
  if (isSyncing) return
  const editor = editorRef.value
  const preview = previewRef.value
  if (!editor || !preview) return
  isSyncing = true
  const topLine = editor.scrollTop / lineHeight()
  const target = interpolate(previewBlocks(), topLine, 'line', 'top')
  preview.scrollTop = target
  requestAnimationFrame(() => { isSyncing = false })
}

/** 预览区滚动 → 同步编辑区 */
function onPreviewScroll() {
  if (isSyncing) return
  const editor = editorRef.value
  const preview = previewRef.value
  if (!editor || !preview) return
  isSyncing = true
  const line = interpolate(previewBlocks(), preview.scrollTop, 'top', 'line')
  editor.scrollTop = line * lineHeight()
  requestAnimationFrame(() => { isSyncing = false })
}
</script>

<template>
  <div
    v-if="group"
    class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800/60"
  >
    <!-- 标题行：点击折叠/展开 -->
    <div
      class="flex items-center justify-between px-4 py-2.5 cursor-pointer select-none"
      @click="toggle"
    >
      <div class="flex items-center gap-2 min-w-0">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="w-3 h-3 text-gray-400 transition-transform flex-shrink-0"
          :class="{ 'rotate-90': expanded }"
          fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"
        >
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
        </svg>
        <span
          v-if="group.color"
          class="w-2 h-2 rounded-full flex-shrink-0"
          :style="{ backgroundColor: group.color }"
        ></span>
        <span class="text-sm font-medium text-gray-700 dark:text-gray-200 truncate flex-shrink-0">
          {{ group.name }}
        </span>
        <!-- 「分组说明」做成徽章，与后面的摘要内容拉开区分 -->
        <span
          class="text-[11px] leading-none px-1.5 py-0.5 rounded flex-shrink-0
                 bg-gray-100 text-gray-500 dark:bg-gray-700 dark:text-gray-300"
        >分组说明</span>
        <!-- 折叠时显示单行摘要：竖线分隔，颜色区别于徽章 -->
        <template v-if="!expanded">
          <span class="w-px h-3 bg-gray-200 dark:bg-gray-600 flex-shrink-0"></span>
          <span
            class="text-xs truncate min-w-0"
            :class="plainSummary
              ? 'text-gray-600 dark:text-gray-300'
              : 'text-gray-300 dark:text-gray-600 italic'"
          >
            {{ plainSummary || '这里什么都没有' }}
          </span>
        </template>
      </div>
      <button
        v-if="expanded && !editing"
        class="text-xs text-gray-400 hover:text-blue-500 dark:hover:text-blue-400 flex-shrink-0"
        @click.stop="startEdit"
      >
        编辑
      </button>
    </div>

    <!-- 展开内容 -->
    <div v-if="expanded" class="px-4 pb-3 pt-0.5 border-t border-gray-100 dark:border-gray-700/60">
      <!-- 展示态 -->
      <template v-if="!editing">
        <div
          v-if="group.description"
          class="md-body text-sm text-gray-600 dark:text-gray-300 break-words"
          v-html="renderedHtml"
        ></div>
        <p v-else class="text-sm text-gray-400 dark:text-gray-500 italic pt-1">这里什么都没有</p>
      </template>

      <!-- 编辑态：左编辑 / 右预览，按内容滚动同步 -->
      <div v-else class="flex flex-col gap-2 pt-2">
        <div class="grid grid-cols-2 gap-3">
          <!-- 编辑区 -->
          <div class="flex flex-col">
            <div class="text-xs text-gray-400 dark:text-gray-500 mb-1">编辑（Markdown）</div>
            <textarea
              ref="editorRef"
              v-model="draft"
              class="md-editor w-full rounded border border-gray-200 dark:border-gray-700
                     bg-gray-50 dark:bg-gray-900/40 text-gray-700 dark:text-gray-200
                     p-2.5 resize-none outline-none focus:border-blue-400 dark:focus:border-blue-500"
              placeholder="输入分组说明，支持 Markdown"
              maxlength="2000"
              spellcheck="false"
              @scroll="onEditorScroll"
            ></textarea>
          </div>
          <!-- 预览区 -->
          <div class="flex flex-col">
            <div class="text-xs text-gray-400 dark:text-gray-500 mb-1">预览</div>
            <div
              ref="previewRef"
              class="md-body md-preview w-full rounded border border-gray-200 dark:border-gray-700
                     bg-white dark:bg-gray-800/40 text-sm text-gray-600 dark:text-gray-300
                     p-2.5 break-words overflow-y-auto"
              @scroll="onPreviewScroll"
              v-html="previewHtml"
            ></div>
          </div>
        </div>
        <div class="flex items-center justify-between">
          <span class="text-xs text-gray-400 dark:text-gray-500">
            支持 Markdown 语法 · {{ draft.length }}/2000
          </span>
          <div class="flex items-center gap-2">
            <el-button size="small" @click="cancelEdit">取消</el-button>
            <el-button size="small" type="primary" :loading="saving" @click="save">保存</el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 编辑区与预览区等高、可滚动，行高固定以便按行号精确换算滚动位置 */
.md-editor {
  height: 320px;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
  line-height: 20px;
  overflow-y: auto;
}
.md-preview {
  height: 320px;
  position: relative; /* 子块 offsetTop 相对此容器计算 */
}
/* 预览块之间保持自然间距（渲染注入的包裹层） */
.md-body :deep(.md-block) { margin: 0; }

/* Markdown 渲染内容的基础排版 */
.md-body :deep(h1),
.md-body :deep(h2),
.md-body :deep(h3) {
  font-weight: 600;
  margin: 0.6em 0 0.3em;
  line-height: 1.3;
}
.md-body :deep(h1) { font-size: 1.15em; }
.md-body :deep(h2) { font-size: 1.08em; }
.md-body :deep(h3) { font-size: 1em; }
.md-body :deep(p) { margin: 0.4em 0; line-height: 1.6; }
.md-body :deep(ul),
.md-body :deep(ol) { margin: 0.4em 0; padding-left: 1.4em; }
.md-body :deep(li) { margin: 0.15em 0; }
.md-body :deep(ul) { list-style: disc; }
.md-body :deep(ol) { list-style: decimal; }
.md-body :deep(a) { color: #3b82f6; text-decoration: underline; }
.md-body :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 0.9em;
  background: rgba(0, 0, 0, 0.06);
  padding: 0.1em 0.35em;
  border-radius: 4px;
}
.dark .md-body :deep(code) { background: rgba(255, 255, 255, 0.1); }
.md-body :deep(pre) {
  background: rgba(0, 0, 0, 0.06);
  padding: 0.7em 0.9em;
  border-radius: 6px;
  overflow-x: auto;
  margin: 0.5em 0;
}
.dark .md-body :deep(pre) { background: rgba(255, 255, 255, 0.08); }
.md-body :deep(pre code) { background: none; padding: 0; }
.md-body :deep(blockquote) {
  border-left: 3px solid #d1d5db;
  padding-left: 0.8em;
  color: #6b7280;
  margin: 0.5em 0;
}
.md-body :deep(hr) {
  border: none;
  border-top: 1px solid #e5e7eb;
  margin: 0.8em 0;
}
.dark .md-body :deep(hr) { border-top-color: #374151; }
.md-body :deep(table) { border-collapse: collapse; margin: 0.5em 0; }
.md-body :deep(th),
.md-body :deep(td) {
  border: 1px solid #e5e7eb;
  padding: 0.3em 0.6em;
}
.dark .md-body :deep(th),
.dark .md-body :deep(td) { border-color: #374151; }
</style>
