<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { PhDatabase, PhGearSix, PhListChecks, PhPulse } from '@phosphor-icons/vue'

const route = useRoute()
const navigation = [
  { name: 'tasks', label: '同步任务', to: '/tasks', icon: PhListChecks },
  { name: 'connections', label: '连接管理', to: '/connections', icon: PhDatabase },
  { name: 'system', label: '系统信息', to: '/system', icon: PhGearSix },
]
const activeName = computed(() => route.name?.toString().startsWith('task') ? 'tasks' : route.name)
</script>

<template>
  <a-config-provider
    :theme="{
      token: {
        colorPrimary: '#34705f',
        colorInfo: '#34705f',
        colorText: '#17211f',
        colorTextSecondary: '#66736f',
        colorBorder: '#dce3e0',
        borderRadius: 10,
        fontFamily: 'Geist Variable, Geist, system-ui, sans-serif',
        controlHeight: 38,
      },
    }"
  >
    <div class="app-shell">
      <aside class="app-sidebar">
        <router-link class="brand" to="/tasks" aria-label="RedisShake Console">
          <span class="brand-mark"><PhPulse :size="22" weight="bold" /></span>
          <span>
            <strong>RedisShake</strong>
            <small>Control plane</small>
          </span>
        </router-link>

        <nav class="primary-nav" aria-label="主导航">
          <router-link
            v-for="item in navigation"
            :key="item.name"
            :to="item.to"
            :aria-label="item.label"
            class="nav-item"
            :class="{ active: activeName === item.name }"
          >
            <component :is="item.icon" :size="19" :weight="activeName === item.name ? 'fill' : 'regular'" />
            <span>{{ item.label }}</span>
          </router-link>
        </nav>

        <div class="sidebar-foot">
          <span class="status-beacon" />
          <div><strong>本地控制面</strong><small>仅监听受控网络</small></div>
        </div>
      </aside>

      <main class="app-main">
        <router-view v-slot="{ Component }">
          <transition name="page" mode="out-in">
            <component :is="Component" />
          </transition>
        </router-view>
      </main>
    </div>
  </a-config-provider>
</template>
