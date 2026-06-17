<script setup lang="ts">
import { ref, watch, computed, onMounted } from 'vue'
import { useAliasStore } from '@/stores/alias'
import { useUiStore } from '@/stores/ui'

/** 路径输入组件 - 支持别名选择或手动输入 */
const props = defineProps<{
  modelValue: string
  /** 当前选中的alias_id（别名模式时） */
  aliasId?: string
  /** 排除的别名 ID 列表（已选中的不再显示） */
  excludeAliasIds?: string[]
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'update:aliasId', value: string | undefined): void
  (e: 'update:mode', value: 'alias' | 'manual'): void
}>()

const aliasStore = useAliasStore()
const uiStore = useUiStore()

// Config 模块: 手动输入的路径必须以 .json/.jsonc/.yaml/.yml/.toml 结尾
const manualPlaceholder = computed(() =>
  uiStore.currentType === 'config'
    ? '输入配置文件路径,如 ~/Library/.../config.json'
    : '输入目标路径,如 /path/to/target'
)

// 模式:alias=选择别名, manual=手动输入
const mode = ref<'alias' | 'manual'>('alias')
// 本地值
const localPath = ref(props.modelValue)

/** 过滤后的别名列表(排除已选) */
const filteredAliases = computed(() => {
  const excludes = props.excludeAliasIds || []
  if (excludes.length === 0) return aliasStore.aliases
  return aliasStore.aliases.filter(a => !excludes.includes(a.id))
})

onMounted(() => {
  if (aliasStore.aliases.length === 0) {
    aliasStore.fetchAliases()
  }
})

watch(() => props.modelValue, (val) => {
  localPath.value = val
})

/** 别名选择变化 */
function handleAliasChange(val: string) {
  const alias = aliasStore.aliases.find(a => a.id === val)
  if (alias) {
    emit('update:modelValue', alias.path)
    emit('update:aliasId', alias.id)
  }
}

/** 手动输入变化 */
function handlePathInput(val: string) {
  localPath.value = val
  emit('update:modelValue', val)
  emit('update:aliasId', undefined)
}

/** 切换模式 */
function handleModeChange(val: string | number | boolean) {
  mode.value = val as 'alias' | 'manual'
  emit('update:mode', val as 'alias' | 'manual')
  if (val === 'manual') {
    emit('update:aliasId', undefined)
  }
}
</script>

<template>
  <div class="flex flex-col gap-2">
    <el-radio-group :model-value="mode" @change="handleModeChange" size="small">
      <el-radio-button value="alias">选择别名</el-radio-button>
      <el-radio-button value="manual">输入路径</el-radio-button>
    </el-radio-group>

    <!-- 别名模式 -->
    <el-select
      v-if="mode === 'alias'"
      :model-value="props.aliasId || ''"
      placeholder="选择路径别名"
      filterable
      class="w-full"
      @change="handleAliasChange"
    >
      <el-option
        v-for="alias in filteredAliases"
        :key="alias.id"
        :label="`${alias.name} (${alias.path})`"
        :value="alias.id"
      />
    </el-select>

    <!-- 手动输入模式 -->
    <el-input
      v-else
      :model-value="localPath"
      :placeholder="manualPlaceholder"
      @input="handlePathInput"
    />
  </div>
</template>
