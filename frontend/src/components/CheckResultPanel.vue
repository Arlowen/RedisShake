<script setup lang="ts">
import { PhCheckCircle, PhWarning, PhXCircle } from '@phosphor-icons/vue'
import type { CheckItem } from '@/api/types'
import StatusPill from '@/components/StatusPill.vue'
import { checkStateMeta } from '@/utils/presentation'

defineProps<{ checks: CheckItem[]; title?: string }>()
const icons = { PASS: PhCheckCircle, WARNING: PhWarning, FAIL: PhXCircle }
</script>

<template>
  <section class="check-panel">
    <header v-if="title"><h3>{{ title }}</h3><span>{{ checks.length }} 项</span></header>
    <div class="check-list">
      <div v-for="item in checks" :key="item.code" class="check-item">
        <component :is="icons[item.state]" :size="20" :weight="item.state === 'PASS' ? 'fill' : 'regular'" />
        <span>{{ item.message }}</span>
        <StatusPill :label="checkStateMeta[item.state].label" :tone="checkStateMeta[item.state].tone" />
      </div>
    </div>
  </section>
</template>

<style scoped>
.check-panel { border: 1px solid var(--line); border-radius: 12px; overflow: hidden; background: #fff; }
header { display: flex; justify-content: space-between; padding: 13px 16px; border-bottom: 1px solid var(--line); }
h3 { margin: 0; font-size: 14px; } header > span { color: var(--muted); font-size: 11px; }
.check-list { display: grid; }
.check-item { display: grid; grid-template-columns: 22px 1fr auto; align-items: center; gap: 10px; min-height: 48px; padding: 9px 14px; border-bottom: 1px solid #edf1ef; color: var(--muted); font-size: 12px; }
.check-item:last-child { border-bottom: 0; }
.check-item > svg { color: var(--accent); }
</style>
