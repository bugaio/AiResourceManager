<template>
  <div class="flex flex-col gap-1">
    <!-- 全部 -->
    <div
      class="flex items-center px-3 py-2 rounded-md cursor-pointer text-sm transition-colors"
      :class="
        resourceStore.currentGroupId === '0'
          ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-300'
          : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
      "
      @click="resourceStore.setGroupId('0')"
    >
      全部
    </div>

    <!-- 分组列表 -->
    <GroupItem
      v-for="group in groupStore.groups"
      :key="group.id"
      :group="group"
      :is-active="resourceStore.currentGroupId === group.id"
      @click="resourceStore.setGroupId(group.id)"
      @rename="handleRename"
      @delete="handleDelete"
    />

    <!-- 加载中 -->
    <div v-if="groupStore.loading" class="text-xs text-gray-400 dark:text-gray-500 px-3 py-1">
      加载中...
    </div>
  </div>

  <!-- 重命名对话框 -->
  <el-dialog v-model="showRenameDialog" title="重命名分组" width="360px">
    <el-input v-model="renameValue" placeholder="输入新名称" @keyup.enter="confirmRename" />
    <template #footer>
      <el-button @click="showRenameDialog = false">取消</el-button>
      <el-button type="primary" :disabled="!renameValue.trim()" @click="confirmRename"
        >确定</el-button
      >
    </template>
  </el-dialog>

  <!-- 删除确认对话框 -->
  <el-dialog v-model="showDeleteDialog" title="删除分组" width="360px">
    <p>确定要删除分组「{{ deleteTarget?.name }}」吗？</p>
    <template #footer>
      <el-button @click="showDeleteDialog = false">取消</el-button>
      <el-button type="danger" @click="confirmDelete">删除</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
/** 分组列表组件 - 显示全部 + 各分组项，管理重命名/删除 */
import { ref, onMounted } from "vue";
import { ElMessage } from "element-plus";
import { useGroupStore } from "@/stores/group";
import { useResourceStore } from "@/stores/resource";
import GroupItem from "./GroupItem.vue";
import type { Group } from "@/types/group";

const groupStore = useGroupStore();
const resourceStore = useResourceStore();

// 重命名对话框
const showRenameDialog = ref(false);
const renameTarget = ref<Group | null>(null);
const renameValue = ref("");

// 删除确认
const showDeleteDialog = ref(false);
const deleteTarget = ref<Group | null>(null);

/** 打开重命名对话框 */
function handleRename(group: Group) {
  renameTarget.value = group;
  renameValue.value = group.name;
  showRenameDialog.value = true;
}

/** 确认重命名 */
async function confirmRename() {
  if (!renameTarget.value) return;
  const name = renameValue.value.trim();
  if (!name || name === renameTarget.value.name) {
    showRenameDialog.value = false;
    return;
  }
  try {
    await groupStore.updateGroup(renameTarget.value.id, { name });
    ElMessage.success("重命名成功");
  } catch (e: any) {
    ElMessage.error(e?.message || "重命名失败");
  }
  showRenameDialog.value = false;
}

/** 打开删除确认 */
function handleDelete(group: Group) {
  deleteTarget.value = group;
  showDeleteDialog.value = true;
}

/** 确认删除 */
async function confirmDelete() {
  if (!deleteTarget.value) return;
  try {
    await groupStore.deleteGroup(deleteTarget.value.id);
    if (resourceStore.currentGroupId === deleteTarget.value.id) {
      resourceStore.setGroupId("0");
    }
    ElMessage.success("分组已删除");
  } catch (e: any) {
    ElMessage.error(e?.message || "删除失败");
  }
  showDeleteDialog.value = false;
}

onMounted(() => {
  groupStore.fetchGroups();
});
</script>
