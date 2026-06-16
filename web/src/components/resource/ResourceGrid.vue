<script setup lang="ts">
import ResourceCard from './ResourceCard.vue'
import { useResourceStore } from '@/stores/resource'
import type { Resource } from '@/types/resource'

/** 资源网格视图 */
const resourceStore = useResourceStore()

const emit = defineEmits<{
  (e: 'edit', resource: Resource): void
  (e: 'editContent', resource: Resource): void
  (e: 'deploy', resource: Resource): void
  (e: 'delete', resource: Resource): void
  (e: 'removeFromGroup', resource: Resource): void
}>()
</script>

<template>
  <!-- 响应式网格布局 -->
  <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
    <ResourceCard
      v-for="item in resourceStore.resources"
      :key="item.id"
      :resource="item"
      @edit="emit('edit', $event)"
      @edit-content="emit('editContent', $event)"
      @deploy="emit('deploy', $event)"
      @remove-from-group="emit('removeFromGroup', $event)"
      @delete="emit('delete', $event)"
    />
  </div>
</template>
