<script setup lang="ts">
/** 编辑私有资源信息（名称 + 描述）
 *
 * 表单与「新增私有资源」一致，打开时回填当前 name / description，
 * 提交调用 updateResource 更新资源元信息。
 */
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { updateResource } from '@/api/resource'
import type { PresetResource } from '@/types/preset'

const props = defineProps<{
  visible: boolean
  resource: PresetResource | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'success'): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const formRef = ref<FormInstance>()
const submitting = ref(false)
const form = reactive({ name: '', description: '' })

const TYPE_LABEL: Record<string, string> = {
  skill: 'Skill', agent: 'SubAgent', config: 'Config', prompt: 'Prompt',
}

const rules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
}

watch(
  () => props.visible,
  (val) => {
    if (val && props.resource) {
      form.name = props.resource.name || ''
      form.description = props.resource.description || ''
    }
  },
)

async function handleSubmit() {
  if (!formRef.value || !props.resource) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    await updateResource(props.resource.id, {
      name: form.name.trim(),
      description: form.description.trim() || undefined,
    })
    ElMessage.success('已保存')
    emit('success')
    emit('update:visible', false)
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    :title="`编辑${TYPE_LABEL[resource?.type || ''] || ''}`"
    width="480px"
    :close-on-click-modal="false"
    append-to-body
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
      <el-form-item label="名称" prop="name">
        <el-input v-model="form.name" placeholder="请输入名称" maxlength="100" />
      </el-form-item>
      <el-form-item label="描述">
        <el-input
          v-model="form.description"
          type="textarea"
          resize="none"
          :rows="8"
          placeholder="可选，简要描述资源用途"
          maxlength="2000"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
    </template>
  </el-dialog>
</template>
