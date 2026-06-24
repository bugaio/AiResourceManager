<script setup lang="ts">
/** Preset 创建/编辑表单对话框 */
import { ref, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { usePresetStore } from '@/stores/preset'
import type { Preset } from '@/types/preset'

const props = defineProps<{
  visible: boolean
  mode: 'create' | 'edit'
  preset?: Preset | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'success', preset: Preset): void
}>()

const store = usePresetStore()
const formRef = ref<FormInstance>()
const submitting = ref(false)

const form = reactive({ name: '', description: '' })

const rules: FormRules = {
  name: [{ required: true, message: '请输入 Preset 名称', trigger: 'blur' }],
}

watch(
  () => props.visible,
  (val) => {
    if (!val) return
    if (props.mode === 'edit' && props.preset) {
      form.name = props.preset.name
      form.description = props.preset.description || ''
    } else {
      form.name = ''
      form.description = ''
    }
  },
)

async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    if (props.mode === 'create') {
      const p = await store.createPreset({
        name: form.name.trim(),
        description: form.description.trim() || undefined,
      })
      ElMessage.success('Preset 创建成功')
      emit('success', p)
    } else if (props.preset) {
      await store.updatePreset(props.preset.id, {
        name: form.name.trim(),
        description: form.description.trim(),
      })
      ElMessage.success('Preset 更新成功')
      emit('success', { ...props.preset, name: form.name.trim(), description: form.description.trim() })
    }
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
    :title="mode === 'create' ? '新建 Preset' : '编辑 Preset'"
    width="440px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
      <el-form-item label="名称" prop="name">
        <el-input v-model="form.name" placeholder="输入 Preset 名称" maxlength="50" />
      </el-form-item>
      <el-form-item label="描述">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="3"
          placeholder="可选,简要描述用途"
          maxlength="500"
        />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:visible', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">
        {{ mode === 'create' ? '创建' : '保存' }}
      </el-button>
    </template>
  </el-dialog>
</template>
