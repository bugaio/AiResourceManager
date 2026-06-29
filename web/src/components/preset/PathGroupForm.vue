<script setup lang="ts">
/** 路径组创建/编辑表单对话框（config 支持多条路径） */
import { ref, reactive, watch, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { usePathGroupStore } from '@/stores/pathGroup'
import { isConfigFilePath, isDirectoryPath, isPromptFilePath } from '@/utils/pathFormat'
import { isRandomGroupName } from '@/utils/pathFormat'
import { summarizeDeploymentsAtPaths, undeployAtPaths } from '@/api/deploy'
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
  config_paths: [''] as string[],
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

const TYPE_LABEL: Record<string, string> = {
  skill: 'Skill', agent: 'Agent', config: 'Config', prompt: 'Prompt',
}
function isRequired(type: string): boolean {
  return !!props.requiredTypes?.includes(type)
}

/** 增删 config 路径输入框 */
function addConfigPath() {
  form.config_paths.push('')
}
function removeConfigPath(idx: number) {
  form.config_paths.splice(idx, 1)
  if (form.config_paths.length === 0) form.config_paths.push('')
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
      const cps = props.pathGroup.config_paths && props.pathGroup.config_paths.length > 0
        ? [...props.pathGroup.config_paths]
        : (props.pathGroup.config_path ? [props.pathGroup.config_path] : [])
      form.config_paths = cps.length > 0 ? cps : ['']
      form.prompt_path = props.pathGroup.prompt_path
    } else {
      form.name = ''
      form.skill_path = ''
      form.agent_path = ''
      form.config_paths = ['']
      form.prompt_path = ''
    }
  },
)

/** 校验子路径，返回错误消息（空串表示通过） */
function validatePaths(): string {
  const skill = form.skill_path.trim()
  const agent = form.agent_path.trim()
  const prompt = form.prompt_path.trim()
  const configs = form.config_paths.map((p) => p.trim()).filter(Boolean)

  if (!skill && !agent && configs.length === 0 && !prompt) return '至少需要填写一个子路径'
  if (skill && !isDirectoryPath(skill)) return 'skill_path 必须是目录路径（不能带文件后缀）'
  if (agent && !isDirectoryPath(agent)) return 'agent_path 必须是目录路径（不能带文件后缀）'
  if (prompt && !isPromptFilePath(prompt)) return 'prompt_path 后缀必须是 .md'
  for (const c of configs) {
    if (!isConfigFilePath(c)) return 'config 路径后缀必须是 .json/.jsonc/.yaml/.yml/.toml'
  }
  // config 路径去重检测
  if (new Set(configs).size !== configs.length) return 'config 路径有重复'
  return ''
}

async function handleSubmit() {
  if (!formRef.value) return

  // 补全「未配置」场景：被要求必填的类型子路径不得为空
  for (const t of props.requiredTypes || []) {
    if (t === 'config') {
      if (form.config_paths.every((p) => !p.trim())) {
        ElMessage.warning('请填写 Config 路径')
        return
      }
      continue
    }
    const key = (t + '_path') as 'skill_path' | 'agent_path' | 'prompt_path'
    if (!form[key].trim()) {
      ElMessage.warning(`请填写 ${TYPE_LABEL[t] || t} 路径`)
      return
    }
  }

  const err = validatePaths()
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

  const configPaths = form.config_paths.map((p) => p.trim()).filter(Boolean)

  // 编辑模式：检测被删除的 config 子路径下是否有已部署内容，有则询问是否移除
  if (props.mode === 'edit' && props.pathGroup) {
    const oldPaths = props.pathGroup.config_paths && props.pathGroup.config_paths.length > 0
      ? props.pathGroup.config_paths
      : (props.pathGroup.config_path ? [props.pathGroup.config_path] : [])
    const removed = oldPaths.filter((p) => p && !configPaths.includes(p))
    if (removed.length > 0) {
      let summary: Record<string, string[]> = {}
      try {
        summary = await summarizeDeploymentsAtPaths(removed)
      } catch {
        summary = {}
      }
      const pathsWithContent = Object.keys(summary)
      if (pathsWithContent.length > 0) {
        const detail = pathsWithContent
          .map((p) => `${p}\n    └ ${summary[p].join('、')}`)
          .join('\n')
        try {
          await ElMessageBox.confirm(
            `以下被删除的 config 路径下有已部署内容，保存将一并移除这些部署：\n\n${detail}`,
            '移除已部署内容',
            {
              confirmButtonText: '确定移除',
              cancelButtonText: '取消',
              type: 'warning',
              customClass: 'whitespace-pre-line',
            },
          )
        } catch {
          return // 用户取消：不保存、不移除
        }
        try {
          await undeployAtPaths(pathsWithContent)
        } catch (e: any) {
          ElMessage.error(e?.message || '移除部署内容失败')
          return
        }
      }
    }
  }

  submitting.value = true
  try {
    if (props.mode === 'create') {
      await store.createPathGroup({
        name: finalName,
        skill_path: form.skill_path.trim(),
        agent_path: form.agent_path.trim(),
        config_paths: configPaths,
        prompt_path: form.prompt_path.trim(),
      })
      ElMessage.success('路径组创建成功')
    } else if (props.pathGroup) {
      await store.updatePathGroup(props.pathGroup.id, {
        name: finalName,
        skill_path: form.skill_path.trim(),
        agent_path: form.agent_path.trim(),
        config_paths: configPaths,
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
      <!-- Config 多路径：每行一个输入框，末行后带「+」可继续添加 -->
      <el-form-item label="Config 路径" :required="isRequired('config')">
        <div class="w-full flex flex-col gap-2">
          <div
            v-for="(_, idx) in form.config_paths"
            :key="idx"
            class="flex items-center gap-2"
          >
            <el-input
              v-model="form.config_paths[idx]"
              placeholder="配置文件 .json/.jsonc/.yaml/.yml/.toml"
            />
            <el-button
              v-if="form.config_paths.length > 1"
              text
              :icon="undefined"
              class="!px-2 text-gray-400 hover:!text-red-500"
              title="删除此路径"
              @click="removeConfigPath(idx)"
            >✕</el-button>
            <el-button
              v-if="idx === form.config_paths.length - 1"
              text
              type="primary"
              class="!px-2"
              title="添加一条 config 路径"
              @click="addConfigPath"
            >＋</el-button>
          </div>
          <p class="text-xs text-gray-400">
            可添加多条 config 路径；部署时为 Preset 中每个 config 资源分别选择目标
          </p>
        </div>
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
