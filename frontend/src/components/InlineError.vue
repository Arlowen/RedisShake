<script setup lang="ts">
import { PhWarningCircle } from '@phosphor-icons/vue'
defineProps<{ message: string }>()
defineEmits<{ retry: [] }>()
</script>

<template>
  <div class="inline-error" role="alert">
    <PhWarningCircle :size="21" weight="fill" />
    <div><strong>数据加载失败</strong><span>{{ message }}</span></div>
    <button type="button" @click="$emit('retry')">重试</button>
  </div>
</template>
