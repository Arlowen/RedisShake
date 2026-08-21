<script setup lang="ts">
defineProps<{ label: string; tone: string; pulse?: boolean }>()
</script>

<template>
  <span class="status-pill" :class="[`tone-${tone}`, { 'is-pulsing': pulse }]">
    <span class="status-pill-dot" />{{ label }}
  </span>
</template>
