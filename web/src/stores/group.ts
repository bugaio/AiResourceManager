import { defineStore } from "pinia";
import { ref, watch } from "vue";
import * as groupApi from "@/api/group";
import { fetchResources } from "@/api/resource";
import { useUiStore } from "./ui";
import { useDeployStore } from "./deploy";
import type { Group } from "@/types/group";

/** 分组状态管理 */
export const useGroupStore = defineStore("group", () => {
  const ui = useUiStore();

  // 分组列表
  const groups = ref<Group[]>([]);
  // "全部"分组的资源数量（该类型全部非私有资源，与"全部"列表口径一致）
  const allCount = ref(0);
  // 加载状态
  const loading = ref(false);

  /** 获取当前类型的分组列表 */
  async function fetchGroups() {
    loading.value = true;
    try {
      // 并行拉：分组列表 + "全部"资源总数（page_size=1 仅取 total）
      const [resp, all] = await Promise.all([
        groupApi.fetchGroups({ type: ui.currentType }),
        fetchResources({ type: ui.currentType, page: 1, page_size: 1 }),
      ]);
      groups.value = resp.list ?? [];
      allCount.value = all.total ?? 0;
    } catch (_e) {
      groups.value = [];
      allCount.value = 0;
    } finally {
      loading.value = false;
    }
  }

  /** 创建分组 */
  async function createGroup(name: string) {
    const group = await groupApi.createGroup({ name, type: ui.currentType });
    groups.value.push(group);
    return group;
  }

  /** 更新分组 */
  async function updateGroup(id: string, data: { name?: string; description?: string; sort_order?: number }) {
    await groupApi.updateGroup(id, data);
    const idx = groups.value.findIndex((g) => g.id === id);
    if (idx !== -1) {
      if (data.name !== undefined) groups.value[idx].name = data.name;
      if (data.description !== undefined) groups.value[idx].description = data.description;
      if (data.sort_order !== undefined) groups.value[idx].sort_order = data.sort_order;
    }
  }

  /** 删除分组 */
  async function deleteGroup(id: string) {
    await groupApi.deleteGroup(id);
    groups.value = groups.value.filter((g) => g.id !== id);
    // 刷新目标路径列表（分组标签需要同步消失）
    useDeployStore().fetchTargets()
  }

  /** 向分组添加资源 */
  async function addResources(groupId: string, resourceIds: string[]) {
    await groupApi.addResources(groupId, resourceIds);
    await fetchGroups();
  }

  /** 从分组移除资源 */
  async function removeResource(groupId: string, resourceId: string) {
    await groupApi.removeResource(groupId, resourceId);
    await fetchGroups();
  }

  /** 批量从分组移除资源（整批仅触发一次同步广播） */
  async function removeResources(groupId: string, resourceIds: string[]) {
    await groupApi.removeResources(groupId, resourceIds);
    await fetchGroups();
  }

  // 监听资源类型切换 → 重新获取分组
  watch(
    () => ui.currentType,
    () => {
      fetchGroups();
    },
  );

  return {
    groups,
    allCount,
    loading,
    fetchGroups,
    createGroup,
    updateGroup,
    deleteGroup,
    addResources,
    removeResource,
    removeResources,
  };
});
