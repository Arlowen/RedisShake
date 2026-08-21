import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/tasks' },
    { path: '/tasks', name: 'tasks', component: () => import('@/views/TasksView.vue') },
    { path: '/tasks/:id', name: 'task-detail', component: () => import('@/views/TaskDetailView.vue') },
    { path: '/connections', name: 'connections', component: () => import('@/views/ConnectionsView.vue') },
    { path: '/system', name: 'system', component: () => import('@/views/SystemView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/tasks' },
  ],
})

export default router
