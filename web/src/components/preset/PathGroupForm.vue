<script setup lang="ts">
/** 路径组创建/编辑表单对话框 */
import { ref, reactive, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { usePathGroupStore } from '@/stores/pathGroup'
import { validatePathGroupPaths } from '@/utils/pathFormat'
import { isRandomGroupName } from '@/utils/pathFormat'
import type { PathGroup } from '@/types/pathGroup'

const props = defineProps<{
  visible: boolean
  mode: 'create' | 'edit'
  pathGroup?: PathGroup | null
  /** 需强制填写的子路径类型（侧栏「未配置」补全场景传入），如 ['config'] */
  requiredTypes?: string[]
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'success'): void
}>()

const store = usePathGroupStore()
const formRef = ref<FormInstance>()
const submitting = ref(false)

const form = reactive({
  name: '',
  skill_path: '',
  agent_path: '',
  config_path: '',
  prompt_path: '',
})

/** 编辑模式下，当前路径组是否为随机名（无真正别名） */
const isRandomName = computed(
  () => props.mode === 'edit' && !!props.pathGroup && isRandomGroupName(props.pathGroup.name),
)
/** 绑定别名开关（仅随机名时出现）；开启后才允许填写真正名称 */
const bindAlias = ref(false)

const rules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
}

/** 某类型是否被要求必填（补全「未配置」场景） */
const TYPE_PATH_KEY: Record<string, 'skill_path' | 'agent_path' | 'config_path' | 'prompt_path'> = {
  skill: 'skill_path', agent: 'agent_path', config: 'config_path', prompt: 'prompt_path',
}
const TYPE_LABEL: Record<string, string> = {
  skill: 'Skill', agent: 'Agent', config: 'Config', prompt: 'Prompt',
}
function isRequired(type: string): boolean {
  return !!props.requiredTypes?.includes(type)
}

watch(
  () => props.visible,
  (val) => {
    if (!val) return
    bindAlias.value = false
    if (props.mode === 'edit' && props.pathGroup) {
      // 随机名路径组：名称暂留空（等用户开启开关再填真名），否则回显原名
      form.name = isRandomGroupName(props.pathGroup.name) ? '' : props.pathGroup.name
      form.skill_path = props.pathGroup.skill_path
      form.agent_path = props.pathGroup.agent_path
      form.config_path = props.pathGroup.config_path
      form.prompt_path = props.pathGroup.prompt_path
    } else {
      form.name = ''
      form.skill_path = ''
      form.agent_path = ''
      form.config_path = ''
      form.prompt_path = ''
    }
  },
)

async function handleSubmit() {
  if (!formRef.value) return

  // 补全「未配置」场景：被要求必填的类型子路径不得为空
  for (const t of props.requiredTypes || []) {
    const key = TYPE_PATH_KEY[t]
    if (key && !form[key].trim()) {
      ElMessage.warning(`请填写 ${TYPE_LABEL[t] || t} 路径`)
      return
    }
  }

  const err = validatePathGroupPaths(form)
  if (err) {
    ElMessage.warning(err)
    return
  }

  // 名称解析：
  // - 创建模式 / 编辑非随机名：名称必填
  // - 编辑随机名：开启绑定别名→新名必填；未开启→保留原随机名
  let finalName = form.name.trim()
  if (isRandomName.value && !bindAlias.value) {
    finalName = props.pathGroup!.name
  } else if (!finalName) {
    ElMessage.warning('请输入名称')
    return
  }

  submitting.value = true
  try {
    if (props.mode === 'create') {
      await store.createPathGroup({
        name: finalName,
        skill_path: form.skill_path.trim(),
        agent_path: form.agent_path.trim(),
        config_path: form.config_path.trim(),
        prompt_path: form.prompt_path.trim(),
      })
      ElMessage.success('路径组创建成功')
    } else if (props.pathGroup) {
      await store.updatePathGroup(props.pathGroup.id, {
        name: finalName,
        skill_path: form.skill_path.trim(),
        agent_path: form.agent_path.trim(),
        config_path: form.config_path.trim(),
        prompt_path: form.prompt_path.trim(),
      })
      ElMessage.success('路径组更新成功')
    }
    emit('success')
    emit('update:visible', false)
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="mode === 'create' ? '新建路径组' : '编辑路径组'"
    width="520px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
  >
    <el-form ref="formRef" :model="form" :rules="isRandomName && !bindAlias ? {} : rules" label-position="top">
      <!-- 名称：随机名路径组默认隐藏，开启绑定别名后才出现 -->
      <el-form-item v-if="!isRandomName || bindAlias" label="名称" prop="name">
        <el-input v-model="form.name" placeholder="输入路径组名称" maxlength="50" />
      </el-form-item>
      <el-form-item label="Skill 路径" :required="isRequired('skill')">
        <el-input v-model="form.skill_path" placeholder="目录路径（支持 ~）" />
      </el-form-item>
      <el-form-item label="Agent 路径" :required="isRequired('agent')">
        <el-input v-model="form.agent_path" placeholder="目录路径（支持 ~）" />
      </el-form-item>
      <el-form-item label="Config 路径" :required="isRequired('config')">
        <el-input
          v-model="form.config_path"
          placeholder="配置文件 .json/.jsonc/.yaml/.yml/.toml"
        />
      </el-form-item>
      <el-form-item label="Prompt 路径" :required="isRequired('prompt')">
        <el-input v-model="form.prompt_path" placeholder="Markdown 文件 .md" />
      </el-form-item>
      <div v-if="requiredTypes && requiredTypes.length > 0" class="text-xs text-amber-500">
        Preset 新增了 {{ requiredTypes.map((t) => TYPE_LABEL[t] || t).join('、') }} 类型资源，请补全对应子路径后保存
      </div>
      <div v-else class="text-xs text-gray-400">至少填写一个子路径</div>

      <!-- 随机名路径组：绑定别名开关 -->
      <div v-if="isRandomName" class="flex items-center gap-2 mt-3 pt-3 border-t border-gray-100 dark:border-gray-700">
        <el-switch v-model="bindAlias" />
        <span class="text-sm text-gray-600 dark:text-gray-400">绑定别名（为此路径组设置一个正式名称）</span>
      </div>
      <p v-if="isRandomName && !bindAlias" class="text-xs text-gray-400 mt-1">
        当前为随机组名，开启上方开关可绑定别名
      </p>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">
        {{ mode === 'create' ? '创建' : '保存' }}
      </el-button>
    </template>
  </el-dialog>
</template>
