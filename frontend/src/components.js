import { checkStateMeta, escapeHtml } from './lib.js'

export function icon(name, size = 18) {
  const paths = {
    search: '<circle cx="11" cy="11" r="7"></circle><path d="m20 20-4-4"></path>',
    refresh: '<path d="M20 6v5h-5"></path><path d="M4 18v-5h5"></path><path d="M6.1 9a7 7 0 0 1 11.2-2.6L20 11M4 13l2.7 4.6A7 7 0 0 0 17.9 15"></path>',
    plus: '<path d="M12 5v14M5 12h14"></path>',
    arrow: '<path d="M5 12h14M13 6l6 6-6 6"></path>',
    back: '<path d="m15 18-6-6 6-6"></path>',
    menu: '<path d="M4 7h16M4 12h16M4 17h16"></path>',
    close: '<path d="m6 6 12 12M18 6 6 18"></path>',
    sun: '<circle cx="12" cy="12" r="4"></circle><path d="M12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4"></path>',
    check: '<path d="m5 12 4 4L19 6"></path>',
    warning: '<path d="M12 3 2.8 20h18.4L12 3Z"></path><path d="M12 9v4M12 17h.01"></path>',
    play: '<path d="m8 5 11 7-11 7V5Z"></path>',
    stop: '<rect x="6" y="6" width="12" height="12" rx="1"></rect>',
    copy: '<rect x="8" y="8" width="11" height="11" rx="2"></rect><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"></path>',
    trash: '<path d="M4 7h16M9 7V4h6v3M7 7l1 14h8l1-14M10 11v6M14 11v6"></path>',
    chevron: '<path d="m7 9.5 5 5 5-5"></path>',
  }
  return `<svg class="icon" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">${paths[name] || paths.arrow}</svg>`
}

export function button(label, { id = '', tone = 'secondary', iconName = '', type = 'button', disabled = false, extra = '' } = {}) {
  return `<button ${id ? `id="${id}"` : ''} type="${type}" class="button button-${tone}" ${disabled ? 'disabled' : ''} ${extra}>${iconName ? icon(iconName, 16) : ''}<span>${escapeHtml(label)}</span></button>`
}

export function statusPill(state, meta, pulse = false) {
  const [label, tone] = meta[state] || [state || '—', 'neutral']
  return `<span class="status-pill tone-${tone}${pulse ? ' pulse' : ''}"><i></i>${escapeHtml(label)}</span>`
}

export function listToolbar({ searchLabel, searchPlaceholder, filters = '', sort = '', action = '', refreshing = false }) {
  return `<section class="list-toolbar" aria-label="管理工具栏">
    <div class="toolbar-primary">${filters}${searchControl(searchLabel, searchPlaceholder)}</div>
    <div class="toolbar-actions">${sort}${button('刷新', { id: 'refresh-list', tone: 'ghost', iconName: 'refresh', extra: `aria-label="刷新${refreshing ? '中' : ''}" title="刷新"` })}${action}</div>
  </section>`
}

export function searchControl(label, placeholder) {
  return `<div class="search-control" role="search" data-search>
    <label for="list-search">${icon('search', 17)}<span class="sr-only">${escapeHtml(label)}</span><input id="list-search" data-search-input aria-label="${escapeHtml(label)}" aria-keyshortcuts="/" autocomplete="off" spellcheck="false" placeholder="${escapeHtml(placeholder)}"></label>
    <button type="button" class="search-clear" data-search-clear aria-label="清空搜索内容" title="清空">${icon('close', 14)}</button>
    <kbd class="search-shortcut" aria-hidden="true">/</kbd>
  </div>`
}

export function listPage({ toolbar, summary = '', content = '', pagination: paginationHtml = '', error = '' }) {
  return `<div class="list-page">${error ? inlineError(error) : ''}${toolbar}<section class="list-results">${summary}<div class="list-content">${content}</div>${paginationHtml}</section></div>`
}

export function table(headers, rows, className = '', label = '数据列表') {
  return `<section class="data-table ${className}" role="table" aria-label="${escapeHtml(label)}"><div class="table-head" role="row">${headers.map((header) => `<span role="columnheader">${escapeHtml(header)}</span>`).join('')}</div><div class="table-body" role="rowgroup">${rows}</div></section>`
}

export function emptyState(title, description = '') {
  return `<div class="empty-row"><div class="empty-content"><strong>${escapeHtml(title)}</strong>${description ? `<span>${escapeHtml(description)}</span>` : ''}</div></div>`
}

export function pagination(total, page = 1, pageSize = 10) {
  const pages = Math.max(1, Math.ceil(total / pageSize))
  const current = Math.min(Math.max(page, 1), pages)
  const start = total ? (current - 1) * pageSize + 1 : 0
  const end = Math.min(total, current * pageSize)
  return `<nav class="pagination" data-pagination aria-label="列表分页">
    <span class="pagination-summary">共 ${total} 条${total ? ` · ${start}–${end}` : ''}</span>
    <div class="pagination-controls">
      <button type="button" data-page="${current - 1}" aria-label="上一页" ${current === 1 ? 'disabled' : ''}>${icon('back', 14)}</button>
      <span><strong>${current}</strong> / ${pages}</span>
      <button type="button" data-page="${current + 1}" aria-label="下一页" ${current === pages ? 'disabled' : ''}>${icon('arrow', 14)}</button>
    </div>
  </nav>`
}

export function bindPagination(root, callback) {
  root.querySelectorAll('[data-pagination] [data-page]').forEach((control) => control.addEventListener('click', () => callback(Number(control.dataset.page))))
}

export function skeleton(rows = 4) { return `<div class="skeleton-list">${Array.from({ length: rows }, () => '<div class="skeleton-row"></div>').join('')}</div>` }

export function summary(items) { return `<div class="summary-strip">${items.map(([value, label]) => `<span><strong>${escapeHtml(value)}</strong>${escapeHtml(label)}</span>`).join('')}</div>` }

export function inlineError(message) { return `<div class="inline-error" role="alert">${icon('warning')}<span>${escapeHtml(message)}</span>${button('重试', { id: 'retry-page', tone: 'ghost' })}</div>` }

export function pageHeader(title, description, actions = '') {
  return `<header class="page-header"><div><h2>${escapeHtml(title)}</h2><p>${escapeHtml(description)}</p></div><div class="page-actions">${actions}</div></header>`
}

export function field(label, control, help = '') {
  return `<div class="field"><span class="field-label">${escapeHtml(label)}</span>${control}${help ? `<small>${escapeHtml(help)}</small>` : ''}</div>`
}

export function input(id, label, value = '', options = {}) {
  const type = options.type || 'text'
  return `<input id="${id}" name="${id}" type="${type}" value="${escapeHtml(value)}" aria-label="${escapeHtml(label)}" placeholder="${escapeHtml(options.placeholder || '')}" ${options.min !== undefined ? `min="${options.min}"` : ''}>`
}

export function textarea(id, label, value = '', placeholder = '', rows = 3) {
  return `<textarea id="${id}" name="${id}" rows="${rows}" aria-label="${escapeHtml(label)}" placeholder="${escapeHtml(placeholder)}">${escapeHtml(value)}</textarea>`
}

export function bindSearch(root, value, callback) {
  const input = root.querySelector('[data-search-input]')
  if (!input) return
  input.value = value
  input.closest('[data-search]')?.classList.toggle('has-value', Boolean(value))
  input.addEventListener('input', (event) => {
    const nextValue = event.target.value
    const caret = event.target.selectionStart ?? nextValue.length
    callback(nextValue)
    const nextInput = root.querySelector('[data-search-input]')
    if (!nextInput) return
    nextInput.focus({ preventScroll: true })
    nextInput.setSelectionRange(caret, caret)
  })
}

let searchesInitialized = false

function clearSearch(input) {
  if (!input) return
  input.value = ''
  input.closest('[data-search]')?.classList.remove('has-value')
  input.dispatchEvent(new Event('input', { bubbles: true }))
  requestAnimationFrame(() => document.querySelector('[data-search-input]')?.focus())
}

export function initSearches() {
  if (searchesInitialized) return
  searchesInitialized = true
  document.addEventListener('input', (event) => {
    const input = event.target.closest?.('[data-search-input]')
    if (input) input.closest('[data-search]')?.classList.toggle('has-value', Boolean(input.value))
  })
  document.addEventListener('click', (event) => {
    if (!(event.target instanceof Element)) return
    const clear = event.target.closest('[data-search-clear]')
    if (clear) clearSearch(clear.closest('[data-search]')?.querySelector('[data-search-input]'))
  })
  document.addEventListener('keydown', (event) => {
    const target = event.target
    const typing = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target?.isContentEditable
    if (event.key === '/' && !typing && !event.metaKey && !event.ctrlKey && !event.altKey) {
      const input = document.querySelector('[data-search-input]')
      if (input) { event.preventDefault(); input.focus(); input.select() }
      return
    }
    if (event.key === 'Escape' && target instanceof HTMLInputElement && target.matches('[data-search-input]')) {
      if (target.value) { event.preventDefault(); clearSearch(target) }
      else target.blur()
    }
  })
}

export function select(id, label, value, options, { align = 'start', size = 'default' } = {}) {
  const currentValue = String(value ?? '')
  const selected = options.find(([optionValue]) => String(optionValue) === currentValue) || options[0] || ['', '暂无选项']
  const listboxId = `${id}-listbox`
  return `<div class="select-control select-${escapeHtml(size)} select-align-${escapeHtml(align)}" data-select>
    <input id="${escapeHtml(id)}" name="${escapeHtml(id)}" type="hidden" value="${escapeHtml(selected[0])}" data-select-input>
    <button type="button" class="select-trigger" data-select-trigger aria-label="${escapeHtml(label)}" aria-haspopup="listbox" aria-expanded="false" aria-controls="${escapeHtml(listboxId)}">
      <span class="select-value" data-select-value>${escapeHtml(selected[1])}</span>${icon('chevron', 16)}
    </button>
    <div id="${escapeHtml(listboxId)}" class="select-menu" role="listbox" aria-label="${escapeHtml(label)}" aria-hidden="true">
      ${options.map(([optionValue, optionLabel]) => {
        const active = String(optionValue) === String(selected[0])
        return `<button type="button" role="option" tabindex="-1" data-select-option data-value="${escapeHtml(optionValue)}" aria-selected="${active}"><span class="select-check">${icon('check', 15)}</span><span>${escapeHtml(optionLabel)}</span></button>`
      }).join('')}
    </div>
  </div>`
}

let selectsInitialized = false

function closeSelect(control) {
  if (!control) return
  control.classList.remove('open', 'drop-up')
  control.querySelector('[data-select-trigger]')?.setAttribute('aria-expanded', 'false')
  control.querySelector('.select-menu')?.setAttribute('aria-hidden', 'true')
}

function closeSelects(except) {
  document.querySelectorAll('[data-select].open').forEach((control) => { if (control !== except) closeSelect(control) })
}

function openSelect(control, focusOption = false, direction = 1) {
  closeSelects(control)
  const trigger = control.querySelector('[data-select-trigger]')
  const menu = control.querySelector('.select-menu')
  const options = [...control.querySelectorAll('[data-select-option]')]
  const selectedIndex = Math.max(0, options.findIndex((option) => option.getAttribute('aria-selected') === 'true'))
  control.classList.add('open')
  trigger.setAttribute('aria-expanded', 'true')
  menu.setAttribute('aria-hidden', 'false')
  const rect = control.getBoundingClientRect()
  const menuHeight = Math.min(menu.scrollHeight, 268)
  if (window.innerHeight - rect.bottom < menuHeight + 16 && rect.top > menuHeight + 16) control.classList.add('drop-up')
  if (focusOption && options.length) requestAnimationFrame(() => options[direction < 0 ? Math.max(0, selectedIndex) : selectedIndex].focus())
}

function chooseSelect(control, option) {
  const input = control.querySelector('[data-select-input]')
  const trigger = control.querySelector('[data-select-trigger]')
  const nextValue = option.dataset.value ?? ''
  const changed = input.value !== nextValue
  input.value = nextValue
  control.querySelector('[data-select-value]').textContent = option.textContent.trim()
  control.querySelectorAll('[data-select-option]').forEach((item) => item.setAttribute('aria-selected', String(item === option)))
  closeSelect(control)
  trigger.focus()
  if (changed) input.dispatchEvent(new Event('change', { bubbles: true }))
}

export function initSelects() {
  if (selectsInitialized) return
  selectsInitialized = true
  document.addEventListener('click', (event) => {
    if (!(event.target instanceof Element)) return
    const option = event.target.closest('[data-select-option]')
    if (option) { chooseSelect(option.closest('[data-select]'), option); return }
    const trigger = event.target.closest('[data-select-trigger]')
    if (trigger) {
      const control = trigger.closest('[data-select]')
      control.classList.contains('open') ? closeSelect(control) : openSelect(control)
      return
    }
    closeSelects()
  })
  document.addEventListener('keydown', (event) => {
    if (!(event.target instanceof Element)) return
    const trigger = event.target.closest('[data-select-trigger]')
    if (trigger) {
      const control = trigger.closest('[data-select]')
      if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); openSelect(control, true, event.key === 'ArrowUp' ? -1 : 1) }
      else if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); control.classList.contains('open') ? closeSelect(control) : openSelect(control, true) }
      else if (event.key === 'Escape') closeSelect(control)
      return
    }
    const option = event.target.closest('[data-select-option]')
    if (!option) return
    const control = option.closest('[data-select]')
    const options = [...control.querySelectorAll('[data-select-option]')]
    const index = options.indexOf(option)
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      options[(index + (event.key === 'ArrowDown' ? 1 : -1) + options.length) % options.length]?.focus()
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault(); options[event.key === 'Home' ? 0 : options.length - 1]?.focus()
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault(); chooseSelect(control, option)
    } else if (event.key === 'Escape') {
      event.preventDefault(); closeSelect(control); control.querySelector('[data-select-trigger]').focus()
    } else if (event.key === 'Tab') closeSelect(control)
  })
}

export function segmented(name, value, options) {
  return `<div class="segmented" role="group">${options.map(([optionValue, label]) => `<button type="button" data-segment="${escapeHtml(name)}" data-value="${escapeHtml(optionValue)}" class="${optionValue === value ? 'active' : ''}">${escapeHtml(label)}</button>`).join('')}</div>`
}

export function checkPanel(checks = [], title = '检查结果') {
  return `<section class="check-panel"><header><h3>${escapeHtml(title)}</h3><span>${checks.length} 项</span></header><div>${checks.map((check) => {
    const [label, tone] = checkStateMeta[check.state] || [check.state, 'neutral']
    return `<div class="check-item tone-text-${tone}">${icon(check.state === 'PASS' ? 'check' : 'warning', 17)}<span>${escapeHtml(check.message)}</span><small>${escapeHtml(label)}</small></div>`
  }).join('')}</div></section>`
}

export function toast(message, tone = 'success') {
  const root = document.querySelector('#toast-root')
  if (!root) return
  const item = document.createElement('div')
  item.className = `toast toast-${tone}`
  item.textContent = message
  root.append(item)
  setTimeout(() => item.remove(), 3200)
}

export function bindSegments(root, callback) {
  root.querySelectorAll('[data-segment]').forEach((control) => control.addEventListener('click', () => callback(control.dataset.segment, control.dataset.value)))
}

export function confirmDialog(title, message, actionLabel = '确认') {
  return new Promise((resolve) => {
    const modal = document.createElement('div')
    modal.className = 'modal-backdrop'
    modal.innerHTML = `<div class="modal"><h3>${escapeHtml(title)}</h3><p>${escapeHtml(message)}</p><div>${button('取消', { id: 'modal-cancel' })}${button(actionLabel, { id: 'modal-confirm', tone: 'primary' })}</div></div>`
    document.body.append(modal)
    modal.querySelector('#modal-cancel').onclick = () => { modal.remove(); resolve(false) }
    modal.querySelector('#modal-confirm').onclick = () => { modal.remove(); resolve(true) }
  })
}
