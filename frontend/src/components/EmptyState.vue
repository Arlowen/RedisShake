<script setup lang="ts">
import { PhFlowArrow } from '@phosphor-icons/vue'
defineProps<{ title: string; description: string }>()
</script>

<template>
  <div class="empty-state">
    <span class="empty-icon"><PhFlowArrow :size="31" /></span>
    <h3>{{ title }}</h3>
    <p>{{ description }}</p>
    <slot />
  </div>
</template>
