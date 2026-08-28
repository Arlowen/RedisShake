import {
  api, clone, defaultConnectionInput, defaultTaskSpec, escapeHtml, formatDate, formatNumber, isActive, lines, modeLabel,
  numbers, runStateMeta, stripAnsi, taskStateMeta, topologyLabel,
} from './lib.js'
import {
  bindPagination, bindSearch, bindSegments, button, checkPanel, confirmDialog, emptyState, field, icon, inlineError, input, listPage, listToolbar,
  pageHeader, pagination, segmented, select, skeleton, statusPill, summary, table, textarea, toast,
} from './components.js'

const LIST_PAGE_SIZE = 10

function setBusy(control, busy, label = '处理中') {
  if (!control) return
  control.disabled = busy
  if (busy) { control.dataset.label = control.textContent; control.textContent = label }
  else if (control.dataset.label) control.textContent = control.dataset.label
}

export async function mountTasks(root, navigate) {
  let tasks = []
  let connections = []
  let latestRuns = {}
  let query = ''
  let sort = 'updated'
  let page = 1
  root.innerHTML = listPage({ toolbar: listToolbar({ searchLabel: '搜索任务', searchPlaceholder: '搜索任务名称、连接或状态' }), content: skeleton(5) })

  const load = async () => {
    try {
      ;[tasks, connections] = await Promise.all([api.listTasks(), api.listConnections()])
      latestRuns = Object.fromEntries(await Promise.all(tasks.map(async (task) => [task.id, (await api.listRuns(task.id))[0]])))
      render()
    } catch (error) { root.innerHTML = inlineError(error.message); root.querySelector('#retry-page')?.addEventListener('click', load) }
  }

  const render = () => {
    const names = Object.fromEntries(connections.map((item) => [item.id, item.name]))
    const keyword = query.trim().toLowerCase()
    const filtered = tasks.filter((task) => {
      const run = latestRuns[task.id]
      const searchable = [task.spec.name, names[task.spec.source_connection_id], names[task.spec.target_connection_id], modeLabel[task.spec.mode], taskStateMeta[task.state]?.[0], runStateMeta[run?.state]?.[0]].filter(Boolean).join(' ').toLowerCase()
      return !keyword || searchable.includes(keyword)
    })
      .sort((a, b) => sort === 'name' ? a.spec.name.localeCompare(b.spec.name) : sort === 'state' ? a.state.localeCompare(b.state) : b.updated_at.localeCompare(a.updated_at))
    page = Math.min(page, Math.max(1, Math.ceil(filtered.length / LIST_PAGE_SIZE)))
    const visibleTasks = filtered.slice((page - 1) * LIST_PAGE_SIZE, page * LIST_PAGE_SIZE)
    const sortControl = select('task-sort', '任务排序', sort, [['updated', '最近更新'], ['name', '任务名称'], ['state', '任务状态']])
    const action = button('创建任务', { id: 'create-task', tone: 'primary', iconName: 'plus' })
    const rows = visibleTasks.map((task) => {
      const run = latestRuns[task.id]
      const route = `${escapeHtml(names[task.spec.source_connection_id] || '未选择')} ${icon('arrow', 14)} ${escapeHtml(names[task.spec.target_connection_id] || '未选择')}`
      return `<article class="table-row task-row" data-task-id="${task.id}">
        <div class="identity"><strong>${escapeHtml(task.spec.name)}</strong><small>${modeLabel[task.spec.mode]} · revision ${task.config_revision}</small></div>
        <div class="route-cell">${route}</div>
        <div class="status-cell">${statusPill(task.state, taskStateMeta)}${run ? statusPill(run.state, runStateMeta, run.state === 'RUNNING') : '<span class="muted">未运行</span>'}</div>
        <time>${formatDate(task.updated_at)}</time>
        <div class="row-actions">${button(task.state === 'DRAFT' ? '继续配置' : '查看', { tone: 'secondary', extra: `data-action="${task.state === 'DRAFT' ? 'edit' : 'view'}" data-id="${task.id}"` })}${button('更多', { tone: 'ghost', extra: `data-action="copy" data-id="${task.id}" aria-label="${escapeHtml(task.spec.name)} 更多操作"` })}</div>
      </article>`
    }).join('')
    const running = Object.values(latestRuns).filter(isActive).length
    const ready = tasks.filter((task) => task.state === 'READY').length
    root.innerHTML = listPage({
      toolbar: listToolbar({ searchLabel: '搜索任务', searchPlaceholder: '搜索任务名称、连接或状态', sort: sortControl, action }),
      summary: tasks.length ? summary([[String(tasks.length), '任务'], [String(running), '运行中'], [String(ready), '可启动']]) : '',
      content: tasks.length ? (filtered.length ? table(['任务名称', '同步链路', '状态', '最近更新', '操作'], rows, 'task-table', '同步任务列表') : emptyState('没有匹配的任务', '请调整搜索条件')) : emptyState('暂无同步任务', '创建任务后，可在这里查看运行状态与迁移进度。'),
      pagination: pagination(filtered.length, page, LIST_PAGE_SIZE),
    })
    bindSearch(root, query, (value) => { query = value; page = 1; render() })
    bindPagination(root, (value) => { page = value; render() })
    root.querySelector('#task-sort').addEventListener('change', (event) => { sort = event.target.value; page = 1; render() })
    root.querySelector('#refresh-list').addEventListener('click', load)
    root.querySelector('#create-task').addEventListener('click', () => navigate('/tasks/new'))
    root.querySelectorAll('[data-action]').forEach((control) => control.addEventListener('click', async () => {
      const task = tasks.find((item) => item.id === control.dataset.id)
      if (control.dataset.action === 'edit') navigate(`/tasks/${task.id}/edit`)
      if (control.dataset.action === 'view') navigate(`/tasks/${task.id}`)
      if (control.dataset.action === 'copy') {
        const name = window.prompt('任务副本名称', `${task.spec.name} 副本`)
        if (name) { await api.copyTask(task.id, name); toast('任务副本已创建'); await load() }
      }
    }))
  }
  await load()
}

export async function mountConnections(root, navigate) {
  let items = []
  let query = ''
  let page = 1
  root.innerHTML = listPage({ toolbar: listToolbar({ searchLabel: '搜索连接', searchPlaceholder: '搜索连接名称、地址或拓扑' }), content: skeleton(4) })
  const load = async () => {
    try { items = await api.listConnections(); render() }
    catch (error) { root.innerHTML = inlineError(error.message); root.querySelector('#retry-page')?.addEventListener('click', load) }
  }
  const render = () => {
    const keyword = query.trim().toLowerCase()
    const filtered = items.filter((item) => {
      const searchable = [item.name, item.address || item.sentinel?.address, topologyLabel[item.topology]].filter(Boolean).join(' ').toLowerCase()
      return !keyword || searchable.includes(keyword)
    })
    page = Math.min(page, Math.max(1, Math.ceil(filtered.length / LIST_PAGE_SIZE)))
    const visibleConnections = filtered.slice((page - 1) * LIST_PAGE_SIZE, page * LIST_PAGE_SIZE)
    const action = button('新建连接', { id: 'create-connection', tone: 'primary', iconName: 'plus' })
    const rows = visibleConnections.map((connection) => `<article class="table-row connection-row">
      <div class="identity"><strong>${escapeHtml(connection.name)}</strong><small class="mono">${escapeHtml(connection.address || connection.sentinel.address || '—')}</small></div>
      <div><small>拓扑</small><strong>${topologyLabel[connection.topology]}</strong></div>
      <div>${statusPill(connection.password_configured ? 'PASS' : 'DRAFT', connection.password_configured ? { PASS: ['凭据已加密', 'success'] } : { DRAFT: ['无密码', 'neutral'] })}</div>
      <time>${formatDate(connection.last_tested_at)}</time>
      <div class="row-actions">${button('测试', { extra: `data-connection-action="test" data-id="${connection.id}"` })}${button('编辑', { tone: 'ghost', extra: `data-connection-action="edit" data-id="${connection.id}"` })}${button('删除', { tone: 'ghost', extra: `data-connection-action="delete" data-id="${connection.id}" aria-label="删除 ${escapeHtml(connection.name)}"` })}</div>
    </article>`).join('')
    root.innerHTML = listPage({
      toolbar: listToolbar({ searchLabel: '搜索连接', searchPlaceholder: '搜索连接名称、地址或拓扑', action }),
      summary: items.length ? summary([[String(items.length), '连接'], ['AES-GCM', '凭据保护']]) : '',
      content: items.length ? (filtered.length ? table(['连接名称 / 地址', '拓扑', '凭据', '最近检查', '操作'], rows, 'connection-table', 'Redis 连接列表') : emptyState('没有匹配的连接', '请调整搜索条件')) : emptyState('暂无 Redis 连接', '先建立源端和目标端连接，再创建同步任务。'),
      pagination: pagination(filtered.length, page, LIST_PAGE_SIZE),
    })
    bindSearch(root, query, (value) => { query = value; page = 1; render() })
    bindPagination(root, (value) => { page = value; render() })
    root.querySelector('#refresh-list').addEventListener('click', load)
    root.querySelector('#create-connection').addEventListener('click', () => navigate('/connections/new'))
    root.querySelectorAll('[data-connection-action]').forEach((control) => control.addEventListener('click', async () => {
      const connection = items.find((item) => item.id === control.dataset.id)
      try {
        if (control.dataset.connectionAction === 'test') {
          setBusy(control, true, '测试中'); const result = await api.testSavedConnection(connection.id, 'source'); toast(result.success ? '连接检查通过' : '连接存在阻断项', result.success ? 'success' : 'warning'); await load()
        }
        if (control.dataset.connectionAction === 'edit') navigate(`/connections/new?edit=${connection.id}`)
        if (control.dataset.connectionAction === 'delete' && await confirmDialog('删除连接？', `“${connection.name}”的凭据删除后无法恢复。`, '删除')) { await api.deleteConnection(connection.id); toast('连接已删除'); await load() }
      } catch (error) { toast(error.message, 'danger'); setBusy(control, false) }
    }))
  }
  await load()
}

export async function mountConnectionEditor(root, navigate) {
  const editId = new URLSearchParams(location.search).get('edit')
  let form = defaultConnectionInput()
  if (editId) {
    const saved = (await api.listConnections()).find((item) => item.id === editId)
    if (saved) form = { ...form, ...saved, password: '', tls: { ...form.tls, ...saved.tls }, sentinel: { ...form.sentinel, ...saved.sentinel, password: '', tls: { ...form.sentinel.tls, ...saved.sentinel.tls } } }
  }
  const standaloneDefaults = defaultConnectionInput()
  form.topology = 'standalone'
  form.sentinel = standaloneDefaults.sentinel
  form.tls = standaloneDefaults.tls
  let purpose = 'source'
  let result
  const render = () => {
    root.innerHTML = `<div class="editor-page connection-editor-page"><div class="editor-layout"><main class="editor-surface">
        <section class="form-section"><header><span>01</span><div><h3>基础连接</h3><p>RedisShake 用这些信息访问真实数据节点</p></div></header>
          <div class="form-grid two">${field('连接名称', input('connection-name', '连接名称', form.name, { placeholder: '例如：生产源端' }))}${field('Redis 地址', input('connection-address', 'Redis 地址', form.address, { placeholder: 'host:port' }))}</div>
          <div class="form-grid two">${field('Redis 用户名', input('connection-username', 'Redis 用户名', form.username, { placeholder: '未启用 ACL 可留空' }))}${field('Redis 密码', input('connection-password', 'Redis 密码', form.password, { type: 'password', placeholder: '未设置密码可留空' }))}</div>
        </section>
      </main><aside class="editor-aside"><div class="aside-card"><span class="eyebrow">连接检查</span><h3>保存前验证</h3><p>源端检查只读；目标写检查会创建带 TTL 的临时 Key 并立即删除。</p>${segmented('test-purpose', purpose, [['source', '源端检查'], ['target', '目标写检查']])}${button('测试连接', { id: 'test-connection', tone: 'primary' })}${result ? checkPanel(result.checks) : ''}</div></aside></div>
      <div class="editor-footer">${button('取消', { id: 'cancel-connection' })}${button(editId ? '保存修改' : '保存连接', { id: 'save-connection', tone: 'primary' })}</div></div>`
    bindSegments(root, (_name, value) => { readConnectionForm(root, form); purpose = value; render() })
    root.querySelector('#cancel-connection').onclick = () => navigate('/connections')
    root.querySelector('#test-connection').onclick = async (event) => {
      try { readConnectionForm(root, form); validateConnection(form); setBusy(event.currentTarget, true, '测试中'); result = await api.testConnection(clone(form), purpose); toast(result.success ? '连接测试通过' : '连接测试存在阻断项', result.success ? 'success' : 'warning'); render() }
      catch (error) { toast(error.message, 'danger'); setBusy(event.currentTarget, false) }
    }
    root.querySelector('#save-connection').onclick = async (event) => {
      try { readConnectionForm(root, form); validateConnection(form); setBusy(event.currentTarget, true, '保存中'); editId ? await api.updateConnection(editId, clone(form)) : await api.createConnection(clone(form)); toast(editId ? '连接已更新' : '连接已创建'); navigate('/connections') }
      catch (error) { toast(error.message, 'danger'); setBusy(event.currentTarget, false) }
    }
  }
  render()
}

function readConnectionForm(root, form) {
  const value = (id, fallback = '') => root.querySelector(`#${id}`)?.value ?? fallback
  form.name = value('connection-name', form.name); form.topology = 'standalone'; form.address = value('connection-address', form.address)
  form.username = value('connection-username', form.username); form.password = value('connection-password', form.password)
}

function validateConnection(form) {
  if (!form.name.trim()) throw new Error('请输入连接名称')
  if (!form.address.trim()) throw new Error('请输入 Redis 地址')
}

export async function mountTaskEditor(root, navigate, taskId = '') {
  let task = taskId ? await api.getTask(taskId) : null
  let spec = task ? clone(task.spec) : defaultTaskSpec()
  let connections = await api.listConnections()
  let precheck = task?.last_precheck_result
  let filterMode = spec.filter.allow_key_prefix?.length || spec.filter.allow_key_regex?.length ? 'allow' : spec.filter.block_key_prefix?.length || spec.filter.block_key_regex?.length ? 'block' : 'none'
  let keyPrefixes = (filterMode === 'allow' ? spec.filter.allow_key_prefix : spec.filter.block_key_prefix).join('\n')
  let keyRegex = (filterMode === 'allow' ? spec.filter.allow_key_regex : spec.filter.block_key_regex).join('\n')
  let databaseIds = (spec.mode === 'scan' ? spec.scan_reader?.dbs : filterMode === 'allow' ? spec.filter.allow_db : spec.filter.block_db)?.join(', ') || ''
  let acknowledge = false

  const syncForm = () => {
    readTaskForm(root, spec)
    keyPrefixes = root.querySelector('#key-prefixes')?.value ?? keyPrefixes
    keyRegex = root.querySelector('#key-regex')?.value ?? keyRegex
    databaseIds = root.querySelector('#database-ids')?.value ?? databaseIds
  }

  const materialize = () => {
    const next = clone(spec)
    next.filter.allow_key_prefix = filterMode === 'allow' ? lines(keyPrefixes) : []; next.filter.block_key_prefix = filterMode === 'block' ? lines(keyPrefixes) : []
    next.filter.allow_key_regex = filterMode === 'allow' ? lines(keyRegex) : []; next.filter.block_key_regex = filterMode === 'block' ? lines(keyRegex) : []
    if (next.mode === 'scan') next.scan_reader.dbs = numbers(databaseIds)
    else { next.filter.allow_db = filterMode === 'allow' ? numbers(databaseIds) : []; next.filter.block_db = filterMode === 'block' ? numbers(databaseIds) : [] }
    return next
  }
  const persist = async () => {
    syncForm(); const next = materialize(); if (!next.name.trim()) throw new Error('请输入任务名称')
    if (!task) task = await api.createTask({ name: next.name, description: next.description, mode: next.mode })
    if (JSON.stringify(task.spec) !== JSON.stringify(next)) task = await api.updateTask(task.id, task.config_revision, next)
    spec = clone(task.spec); return task
  }
  const render = () => {
    const connectionOptions = [['', '请选择连接'], ...connections.map((connection) => [connection.id, connection.name])]
    const hasWarnings = precheck?.checks?.some((item) => item.state === 'WARNING')
    const canStart = Boolean(precheck?.ready && task && JSON.stringify(materialize()) === JSON.stringify(task.spec))
    root.innerHTML = `<div class="editor-page task-editor-page"><button id="back-tasks" class="back-link">${icon('back', 16)}返回任务列表</button>
      ${pageHeader(taskId ? '编辑同步任务' : '创建同步任务', '配置 Redis 同步链路、迁移范围与内核参数。', `${button('取消', { id: 'cancel-task' })}${button('保存草稿', { id: 'save-task' })}${button(canStart ? '启动任务' : '执行预检查', { id: canStart ? 'start-task' : 'precheck-task', tone: 'primary', disabled: hasWarnings && !acknowledge })}`)}
      <div class="editor-layout"><main class="editor-surface">
        <section class="form-section"><header><span>01</span><div><h3>基本信息</h3><p>定义任务名称与 RedisShake reader 模式</p></div></header>
          ${field('任务名称', input('task-name', '任务名称', spec.name, { placeholder: '例如：订单缓存迁移' }))}${field('任务描述', textarea('task-description', '任务描述', spec.description, '可选', 2))}
          ${field('同步模式', `<div class="mode-grid"><button type="button" data-mode="sync" class="mode-card ${spec.mode === 'sync' ? 'active' : ''}"><strong>增量同步</strong><span>RDB + AOF 持续同步</span></button><button type="button" data-mode="scan" class="mode-card ${spec.mode === 'scan' ? 'active' : ''}"><strong>扫描迁移</strong><span>SCAN 一次性迁移</span></button></div>`)}
        </section>
        <section class="form-section"><header><span>02</span><div><h3>同步链路</h3><p>源端与目标端必须使用不同连接</p></div></header>
          <div class="form-grid two">${field('源端连接', select('source-connection', '源端连接', spec.source_connection_id, connectionOptions))}${field('目标连接', select('target-connection', '目标连接', spec.target_connection_id, connectionOptions))}</div>
          <div class="route-preview"><div><small>源端</small><strong>${escapeHtml(connections.find((item) => item.id === spec.source_connection_id)?.name || '未选择')}</strong></div>${icon('arrow')}<div><small>目标端</small><strong>${escapeHtml(connections.find((item) => item.id === spec.target_connection_id)?.name || '未选择')}</strong></div></div>
        </section>
        <section class="form-section"><header><span>03</span><div><h3>同步范围</h3><p>默认迁移全部数据，可按 Key 与 DB 收敛</p></div></header>
          ${field('过滤策略', segmented('filter-mode', filterMode, [['none', '不过滤'], ['allow', '仅允许'], ['block', '排除']]))}
          ${filterMode !== 'none' ? `<div class="form-grid two">${field('Key 前缀', textarea('key-prefixes', 'Key 前缀', keyPrefixes, 'cache:\nsession:', 3))}${field('Key 正则', textarea('key-regex', 'Key 正则', keyRegex, '^order:\\d+$', 3))}</div>` : ''}
          ${field(spec.mode === 'scan' ? '扫描 DB' : 'DB 过滤', input('database-ids', spec.mode === 'scan' ? '扫描 DB' : 'DB 过滤', databaseIds, { placeholder: '例如：0, 1, 5；留空表示全部' }))}
          ${spec.mode === 'scan' ? `<div class="form-grid two">${field('SCAN Count', input('scan-count', 'SCAN Count', spec.scan_reader.count, { type: 'number', min: 1 }))}<label class="switch-row compact"><span>Keyspace Notification</span><input id="scan-ksn" type="checkbox" ${spec.scan_reader.ksn ? 'checked' : ''}></label></div>` : ''}
        </section>
        <details class="advanced"><summary>高级设置</summary><div class="form-grid two">${field('目标最大 QPS', input('target-qps', '目标最大 QPS', spec.advanced.target_redis_max_qps, { type: 'number', min: 1 }))}${field('Pipeline Count', input('pipeline-count', 'Pipeline Count', spec.advanced.pipeline_count_limit, { type: 'number', min: 1 }))}</div>${field('目标 Key 已存在时', select('restore-behavior', '目标 Key 已存在时', spec.advanced.rdb_restore_command_behavior, [['panic', '停止任务（推荐）'], ['rewrite', '覆盖目标 Key'], ['skip', '跳过冲突 Key']]))}<label class="danger-row"><div>${icon('warning')}<span><strong>启动前清空目标 Redis</strong><small>会执行 FLUSHALL，目标端数据不可恢复。</small></span></div><input id="empty-target" type="checkbox" ${spec.advanced.empty_db_before_sync ? 'checked' : ''}></label></details>
      </main><aside class="editor-aside"><div class="aside-card sticky"><span class="eyebrow">任务校验</span><h3>${precheck ? (precheck.ready ? '可以启动' : '需要处理') : '尚未预检查'}</h3><p>控制面会生成真实 RedisShake TOML，并由内核解析器校验。</p>${precheck ? checkPanel(precheck.checks, '预检查结果') : '<div class="aside-placeholder">保存配置后执行预检查</div>'}${hasWarnings ? `<label class="ack-row"><input id="ack-warning" type="checkbox" ${acknowledge ? 'checked' : ''}>我已确认上述危险警告</label>` : ''}</div></aside></div></div>`
    root.querySelector('#back-tasks').onclick = root.querySelector('#cancel-task').onclick = () => navigate('/tasks')
    root.querySelectorAll('[data-mode]').forEach((control) => control.onclick = () => { syncForm(); spec.mode = control.dataset.mode; spec.sync_reader = spec.mode === 'sync' ? (spec.sync_reader || { sync_rdb: true, sync_aof: true, prefer_replica: false, try_diskless: false }) : undefined; spec.scan_reader = spec.mode === 'scan' ? (spec.scan_reader || { dbs: [], scan: true, ksn: false, count: 1, prefer_replica: false, skip_unknown_type: [] }) : undefined; precheck = undefined; render() })
    bindSegments(root, (_name, value) => { syncForm(); filterMode = value; precheck = undefined; render() })
    root.querySelector('#source-connection').onchange = root.querySelector('#target-connection').onchange = () => { readTaskForm(root, spec); precheck = undefined; render() }
    root.querySelector('#ack-warning')?.addEventListener('change', (event) => { acknowledge = event.target.checked; render() })
    root.querySelector('#save-task').onclick = async (event) => { try { setBusy(event.currentTarget, true, '保存中'); await persist(); toast('草稿已保存'); navigate('/tasks') } catch (error) { toast(error.message, 'danger'); setBusy(event.currentTarget, false) } }
    root.querySelector('#precheck-task')?.addEventListener('click', async (event) => {
      try {
        syncForm(); if (!spec.source_connection_id) throw new Error('请选择源端 Redis'); if (!spec.target_connection_id) throw new Error('请选择目标 Redis'); if (spec.source_connection_id === spec.target_connection_id) throw new Error('源端和目标端不能使用同一个连接')
        setBusy(event.currentTarget, true, '检查中'); const saved = await persist(); precheck = await api.precheckTask(saved.id, saved.config_revision, acknowledge); task = await api.getTask(saved.id); spec = clone(task.spec); toast(precheck.ready ? '预检查通过，可以启动' : '请处理预检查结果', precheck.ready ? 'success' : 'warning'); render()
      } catch (error) { toast(error.message, 'danger'); setBusy(event.currentTarget, false) }
    })
    root.querySelector('#start-task')?.addEventListener('click', async (event) => { try { setBusy(event.currentTarget, true, '启动中'); const run = await api.startRun(task.id, task.config_revision); toast(run.state === 'SUCCEEDED' ? '任务已完成' : '同步任务已启动'); navigate(`/tasks/${task.id}`) } catch (error) { toast(error.message, 'danger'); setBusy(event.currentTarget, false) } })
  }
  render()
}

function readTaskForm(root, spec) {
  const value = (id, fallback = '') => root.querySelector(`#${id}`)?.value ?? fallback
  spec.name = value('task-name', spec.name); spec.description = value('task-description', spec.description); spec.source_connection_id = value('source-connection', spec.source_connection_id); spec.target_connection_id = value('target-connection', spec.target_connection_id)
  spec.advanced.target_redis_max_qps = Number(value('target-qps', spec.advanced.target_redis_max_qps)); spec.advanced.pipeline_count_limit = Number(value('pipeline-count', spec.advanced.pipeline_count_limit)); spec.advanced.rdb_restore_command_behavior = value('restore-behavior', spec.advanced.rdb_restore_command_behavior); spec.advanced.empty_db_before_sync = root.querySelector('#empty-target')?.checked || false
  if (spec.mode === 'scan') { spec.scan_reader.count = Number(value('scan-count', spec.scan_reader.count)); spec.scan_reader.ksn = root.querySelector('#scan-ksn')?.checked || false }
}

export async function mountTaskDetail(root, navigate, id) {
  let task
  let runs = []
  let selectedRun
  let tab = 'overview'
  let logs = ''
  let poll
  root.innerHTML = skeleton(5)
  const load = async () => {
    try { ;[task, runs] = await Promise.all([api.getTask(id), api.listRuns(id)]); selectedRun = runs[0]; await render() }
    catch (error) { root.innerHTML = inlineError(error.message); root.querySelector('#retry-page')?.addEventListener('click', load) }
  }
  const render = async () => {
    clearTimeout(poll)
    const metrics = selectedRun?.status?.total_entries_count || {}
    const active = isActive(selectedRun)
    root.innerHTML = `<div class="detail-page"><button id="back-tasks" class="back-link">${icon('back', 16)}返回任务列表</button>
      ${pageHeader(task.spec.name, `${modeLabel[task.spec.mode]} · revision ${task.config_revision} · ${formatDate(task.updated_at)}`, `${button('刷新', { id: 'refresh-detail', iconName: 'refresh' })}${task.state === 'READY' && !active ? button('启动', { id: 'start-detail', tone: 'primary', iconName: 'play' }) : ''}${active ? button('停止', { id: 'stop-detail', tone: 'danger', iconName: 'stop' }) : ''}`)}
      <div class="metric-strip"><div class="metric-status">${statusPill(task.state, taskStateMeta)}${selectedRun ? statusPill(selectedRun.state, runStateMeta, selectedRun.state === 'RUNNING') : '<span class="muted">尚未运行</span>'}</div><div><small>心跳</small><strong>${selectedRun?.status_healthy ? '正常' : selectedRun ? '不可用' : '—'}</strong></div><div><small>读取</small><strong>${formatNumber(metrics.read_count)}</strong></div><div><small>写入</small><strong>${formatNumber(metrics.write_count)}</strong></div><div><small>OPS</small><strong>${formatNumber(metrics.write_ops)}</strong></div></div>
      <nav class="tabs" role="tablist">${[['overview', '运行概览'], ['logs', '运行日志'], ['history', '运行历史'], ['config', '配置快照']].map(([key, label]) => `<button role="tab" aria-selected="${tab === key}" data-tab="${key}" class="${tab === key ? 'active' : ''}">${label}</button>`).join('')}</nav>
      <section id="detail-content">${detailTab()}</section></div>`
    root.querySelector('#back-tasks').onclick = () => navigate('/tasks')
    root.querySelector('#refresh-detail').onclick = load
    root.querySelectorAll('[data-tab]').forEach((control) => control.onclick = async () => { tab = control.dataset.tab; if (tab === 'logs' && selectedRun) { const result = await api.readLogs(selectedRun.id, 0, 131072); logs = stripAnsi(result.content) } await render() })
    root.querySelectorAll('[data-run-id]').forEach((control) => control.onclick = async () => { selectedRun = runs.find((run) => run.id === control.dataset.runId); logs = ''; await render() })
    root.querySelector('#start-detail')?.addEventListener('click', async () => { selectedRun = await api.startRun(task.id, task.config_revision); runs.unshift(selectedRun); toast('任务已启动'); await render() })
    root.querySelector('#stop-detail')?.addEventListener('click', async () => { if (await confirmDialog('停止当前同步？', '控制面会先发送 SIGTERM，并等待 RedisShake 安全退出。', '停止')) { selectedRun = await api.stopRun(selectedRun.id); toast('正在优雅停止'); await load() } })
    if (active) poll = setTimeout(load, 1400)
  }
  const detailTab = () => {
    if (tab === 'logs') return `<div class="log-toolbar"><strong>stdout / stderr（已脱敏）</strong>${button('下载', { id: 'download-logs' })}</div><pre class="log-view">${escapeHtml(logs || '等待日志输出…')}</pre>`
    if (tab === 'history') return runs.length ? table(['状态', '运行 ID', '开始时间', '退出原因'], runs.map((run) => `<article class="table-row history-row">${statusPill(run.state, runStateMeta)}<code>${escapeHtml(run.id)}</code><time>${formatDate(run.started_at)}</time><span>${escapeHtml(run.exit_reason || '—')}</span></article>`).join(''), 'history-table', '运行历史列表') : emptyState('暂无运行记录')
    if (tab === 'config') return `<div class="config-note">配置快照不包含密码或 TLS PEM。</div><pre class="json-view">${escapeHtml(JSON.stringify(selectedRun?.config_snapshot || task.spec, null, 2))}</pre>`
    return `<div class="overview-grid"><main><div class="section-title"><h3>RedisShake 状态</h3><span>来自 worker 状态端口</span></div><div class="status-grid"><div><small>阶段</small><strong>${escapeHtml(String(selectedRun?.status?.reader?.status || selectedRun?.state || '—'))}</strong></div><div><small>内部一致</small><strong>${selectedRun?.status?.consistent === undefined ? '—' : selectedRun.status.consistent ? '是' : '否'}</strong></div><div><small>最后心跳</small><strong>${formatDate(selectedRun?.last_heartbeat_at)}</strong></div></div><pre class="json-view">${escapeHtml(JSON.stringify({ reader: selectedRun?.status?.reader, writer: selectedRun?.status?.writer }, null, 2))}</pre></main><aside class="run-list"><div class="section-title"><h3>运行记录</h3><span>${runs.length}</span></div>${runs.map((run) => `<button data-run-id="${run.id}" class="${run.id === selectedRun?.id ? 'active' : ''}">${statusPill(run.state, runStateMeta)}<strong>${formatDate(run.started_at)}</strong><small>${escapeHtml(run.id.slice(0, 10))}</small></button>`).join('') || '<p>尚未运行</p>'}</aside></div>`
  }
  await load()
  return () => clearTimeout(poll)
}
