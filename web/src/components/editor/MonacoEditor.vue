<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, shallowRef } from 'vue'
import * as monaco from 'monaco-editor'

// 配置 JSON diagnostics：允许注释和 trailing commas（JSONC 支持）
monaco.languages.json.jsonDefaults.setDiagnosticsOptions({
  validate: true,
  allowComments: true,
  trailingCommas: 'ignore',
  schemaValidation: 'ignore',
})

/** Monaco 编辑器封装组件 */
const props = withDefaults(defineProps<{
  modelValue: string
  language?: string
  theme?: 'vs' | 'vs-dark'
  readonly?: boolean
}>(), {
  language: 'markdown',
  theme: 'vs',
  readonly: false,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

// 编辑器容器DOM引用
const containerRef = ref<HTMLDivElement>()
// 编辑器实例（浅引用避免深度响应）
const editorInstance = shallowRef<monaco.editor.IStandaloneCodeEditor>()
// 内部更新标记，防止循环触发
let isInternalUpdate = false

onMounted(() => {
  if (!containerRef.value) return
  // 创建编辑器实例
  editorInstance.value = monaco.editor.create(containerRef.value, {
    value: props.modelValue,
    language: props.language,
    theme: props.theme,
    automaticLayout: true,
    minimap: { enabled: true },
    fontSize: 14,
    lineNumbers: 'on',
    wordWrap: 'on',
    scrollBeyondLastLine: false,
    tabSize: 2,
    readOnly: props.readonly,
  })

  // 监听编辑器内容变化 → 向外emit
  editorInstance.value.onDidChangeModelContent(() => {
    if (isInternalUpdate) return
    const value = editorInstance.value!.getValue()
    emit('update:modelValue', value)
  })
})

onUnmounted(() => {
  // 销毁编辑器实例释放资源
  editorInstance.value?.dispose()
})

// 监听语言变化 → 切换编辑器语言模型
watch(() => props.language, (lang) => {
  const model = editorInstance.value?.getModel()
  if (model) {
    monaco.editor.setModelLanguage(model, lang)
  }
})

// 监听主题变化 → 切换Monaco主题
watch(() => props.theme, (theme) => {
  monaco.editor.setTheme(theme)
})

// 监听只读状态变化 → 更新编辑器只读
watch(() => props.readonly, (val) => {
  editorInstance.value?.updateOptions({ readOnly: val })
})

// 监听外部modelValue变化 → 更新编辑器内容（避免循环）
watch(() => props.modelValue, (newVal) => {
  if (editorInstance.value && editorInstance.value.getValue() !== newVal) {
    isInternalUpdate = true
    editorInstance.value.setValue(newVal)
    isInternalUpdate = false
  }
})

/** 触发 Monaco 内置格式化 */
function format() {
  editorInstance.value?.getAction('editor.action.formatDocument')?.run()
}

defineExpose({ format })
</script>

<template>
  <div ref="containerRef" class="w-full h-full" />
</template>
