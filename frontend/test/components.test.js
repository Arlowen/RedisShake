import test from 'node:test'
import assert from 'node:assert/strict'

import { button, listPage, listToolbar, table } from '../src/components.js'
import { defaultConnectionInput, defaultTaskSpec, escapeHtml } from '../src/lib.js'

test('shared list shell keeps search, actions and table aligned in one component system', () => {
  const toolbar = listToolbar({ searchLabel: '搜索任务', searchPlaceholder: '搜索任务名称', action: button('创建任务', { tone: 'primary' }) })
  const page = listPage({ toolbar, content: table(['名称'], '<article class="table-row">任务</article>') })
  assert.match(page, /class="list-page"/)
  assert.match(page, /aria-label="搜索任务"/)
  assert.match(page, /class="data-table/)
  assert.match(page, /创建任务/)
})

test('defaults provide complete RedisShake connection and task payloads', () => {
  assert.equal(defaultConnectionInput().topology, 'standalone')
  assert.equal(defaultTaskSpec('scan').scan_reader.scan, true)
  assert.equal(defaultTaskSpec('sync').advanced.pipeline_count_limit, 1024)
})

test('HTML output escapes API-provided values', () => {
  assert.equal(escapeHtml('<script>"x"</script>'), '&lt;script&gt;&quot;x&quot;&lt;/script&gt;')
})
