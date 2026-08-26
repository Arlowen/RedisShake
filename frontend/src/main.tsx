import 'antd/dist/reset.css'
import '@/styles/main.css'
import '@/styles/features.css'

import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'

import App from '@/App'
import { initialTheme } from '@/utils/theme'

const bootTheme = initialTheme()
document.documentElement.dataset.theme = bootTheme
document.documentElement.style.colorScheme = bootTheme

const root = document.getElementById('app')
if (!root) throw new Error('RedisShake Console root element is missing')

createRoot(root).render(
  <BrowserRouter>
    <App />
  </BrowserRouter>,
)
