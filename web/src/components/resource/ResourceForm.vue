<script setup lang="ts">
import { ref, watch, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import GroupSelect from '@/components/common/GroupSelect.vue'
import { useUiStore } from '@/stores/ui'
import { useGroupStore } from '@/stores/group'
import { createResource, updateResource } from '@/api/resource'
import type { Resource } from '@/types/resource'

/** 资源创建/编辑表单对话框 */
const props = defineProps<{
  visible: boolean
  mode: 'create' | 'edit'
  resource?: Resource
}>()

const emit = defineEmits<{
  (e: 'update:visible', val: boolean): void
  (e: 'success'): void
}>()

const uiStore = useUiStore()
const groupStore = useGroupStore()

// 表单引用
const formRef = ref<FormInstance>()
// 提交中状态
const submitting = ref(false)

// 表单数据
const form = reactive({
  name: '',
  description: '',
  groupIds: [] as string[],
})

// 表单验证规则
const rules: FormRules = {
  name: [{ required: true, message: '请输入资源名称', trigger: 'blur' }],
}

/** 对话框打开时初始化表单 */
watch(() => props.visible, (val) => {
  if (val) {
    if (props.mode === 'edit' && props.resource) {
      form.name = props.resource.name
      form.description = props.resource.description || ''
      form.groupIds = []
    } else {
      form.name = ''
      form.description = ''
      form.groupIds = []
    }
  }
})

/** 提交表单 */
async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    if (props.mode === 'create') {
      // 创建资源
      const res = await createResource({
        type: uiStore.currentType,
        name: form.name,
        description: form.description || undefined,
      })
      // 如果选择了分组，添加到分组
      for (const gid of form.groupIds) {
        await groupStore.addResources(gid, [res.id])
      }
      ElMessage.success('创建成功')
    } else {
      // 编辑资源
      await updateResource(props.resource!.id, {
        name: form.name,
        description: form.description || undefined,
      })
      ElMessage.success('更新成功')
    }
    emit('success')
    emit('update:visible', false)
  } catch (e: any) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

/** 关闭对话框 */
function handleClose() {
  emit('update:visible', false)
}
</script>

<template>
  <el-dialog
    :model-value="visible"
    :title="mode === 'create' ? '新建资源' : '编辑资源'"
    width="480px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-position="top"
      class="space-y-4"
    >
      <!-- 名称 -->
      <el-form-item label="名称" prop="name">
        <el-input v-model="form.name" placeholder="请输入资源名称" maxlength="100" />
      </el-form-item>

      <!-- 描述 -->
      <el-form-item label="描述" prop="description">
        <el-input
          v-model="form.description"
          type="textarea"
          :rows="3"
          placeholder="可选，简要描述资源用途"
          maxlength="500"
        />
      </el-form-item>

      <!-- 分组选择（仅创建时展示） -->
      <el-form-item v-if="mode === 'create'" label="分组（可选）">
        <GroupSelect v-model="form.groupIds" :type="uiStore.currentType" />
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">
        {{ mode === 'create' ? '创建' : '保存' }}
      </el-button>
    </template>
  </el-dialog>
</template>
