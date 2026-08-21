import '@fontsource-variable/geist'
import 'ant-design-vue/dist/reset.css'
import '@/styles/main.css'

import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from '@/App.vue'
import router from '@/router'

createApp(App).use(createPinia()).use(router).mount('#app')
