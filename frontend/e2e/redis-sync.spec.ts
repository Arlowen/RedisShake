import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import { expect, test } from '@playwright/test'

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const composeArgs = ['compose', '-f', 'deploy/compose.dev.yaml']

function composeExec(service: string, ...command: string[]) {
  return execFileSync('docker', [...composeArgs, 'exec', '-T', service, ...command], {
    cwd: repositoryRoot,
    encoding: 'utf8',
  }).trim()
}

test('creates connections and runs a real RedisShake scan from the UI', async ({ page }) => {
  const suffix = Date.now().toString(36)
  const sourceName = `E2E 源端 ${suffix}`
  const targetName = `E2E 目标端 ${suffix}`
  const taskName = `E2E 扫描迁移 ${suffix}`
  const payloadKey = `e2e:payload:${suffix}`
  const skippedKey = `e2e:skip:${suffix}`
  expect(composeExec('source-redis', 'redis-cli', 'FLUSHALL')).toBe('OK')
  expect(composeExec('target-redis', 'redis-cli', 'FLUSHALL')).toBe('OK')
  expect(composeExec('source-redis', 'redis-cli', 'SET', payloadKey, 'from-playwright')).toBe('OK')
  expect(composeExec('source-redis', 'redis-cli', 'SET', skippedKey, 'filtered')).toBe('OK')

  await page.goto('/connections')
  await createConnection(page, sourceName, 'source-redis:6379', false)
  await createConnection(page, targetName, 'target-redis:6379', true)

  await page.getByRole('link', { name: '同步任务' }).click()
  await page.getByRole('button', { name: '创建任务' }).first().click()
  await expect(page).toHaveURL(/\/tasks\/new$/)
  await expect(page.getByRole('heading', { name: '创建同步任务' })).toBeVisible()
  await page.getByLabel('任务名称', { exact: true }).fill(taskName)
  await page.getByText('扫描迁移', { exact: true }).click()
  await page.getByRole('combobox', { name: /源端连接/ }).click()
  await page.getByText(sourceName, { exact: true }).last().click()
  await page.getByRole('combobox', { name: /目标连接/ }).click()
  await page.getByText(targetName, { exact: true }).last().click()
  await page.getByText('排除', { exact: true }).click()
  await page.getByLabel('Key 前缀', { exact: true }).fill('e2e:skip:')
  await page.getByRole('button', { name: '执行预检查' }).click()
  await expect(page.getByText('RedisShake 配置生成并通过内核解析', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '启动任务' }).click()

  await expect(page.getByRole('heading', { name: taskName })).toBeVisible()
  await expect(page.getByText('已完成', { exact: true }).first()).toBeVisible({ timeout: 20_000 })
  expect(composeExec('target-redis', 'redis-cli', 'GET', payloadKey)).toBe('from-playwright')
  expect(composeExec('target-redis', 'redis-cli', 'EXISTS', skippedKey)).toBe('0')
  await page.getByRole('tab', { name: '运行日志' }).click()
  await expect(page.getByText('stdout / stderr（已脱敏）', { exact: true })).toBeVisible()
  await expect(page.locator('.log-view')).toContainText('all done')
})

async function createConnection(page: import('@playwright/test').Page, name: string, address: string, targetCheck: boolean) {
  await page.getByRole('button', { name: '新建连接' }).click()
  await page.getByLabel('连接名称', { exact: true }).fill(name)
  await page.getByLabel('Redis 地址', { exact: true }).fill(address)
  if (targetCheck) {
    await page.getByText('目标写检查', { exact: true }).click()
  }
  await page.getByRole('button', { name: '测试连接' }).click()
  await expect(page.getByText(targetCheck ? '目标 Redis 临时测试 Key 已清理' : 'Redis 拓扑与连接配置一致', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '保存连接' }).click()
  await expect(page.getByText(name, { exact: true })).toBeVisible()
}
