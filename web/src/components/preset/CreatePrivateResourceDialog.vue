<script setup lang="ts">
/** 统一的「新增私有资源」对话框（4 种类型共用）
 *
 * 表单只有名称 + 描述两个字段。
 * 注意：此弹窗「保存」时不调用后端，仅把一条临时记录回传给 AddResourceDialog，
 * 真正的创建（建文件 + 落库）延迟到 AddResourceDialog 点「确定」时统一执行。
 * 因此弹窗A 取消则这些临时记录直接丢弃，不产生游离资源。
 */
import { computed, reactive, ref, watch } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import type { ResourceType } from '@/types/resource'

const props = defineProps<{
  visible: boolean
  type: ResourceType
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'save', payload: { type: ResourceType; name: string; description: string }): void
}>()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val),
})

const formRef = ref<FormInstance>()
const form = reactive({ name: '', description: '' })

const TYPE_LABEL: Record<ResourceType, string> = {
  skill: 'Skill', agent: 'SubAgent', config: 'Config', prompt: 'Prompt',
}

const rules: FormRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
}

watch(
  () => props.visible,
  (val) => {
    if (val) {
      form.name = ''
      form.description = ''
    }
  },
)

async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  emit('save', {
    type: props.type,
    name: form.name.trim(),
    description: form.description.trim(),
  })
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog
    v-model="dialogVisible"
    :title="`新增私有 ${TYPE_LABEL[type]}`"
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
      <el-button type="primary" @click="handleSubmit">保存</el-button>
    </template>
  </el-dialog>
</template>
