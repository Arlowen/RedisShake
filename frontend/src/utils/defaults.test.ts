import { describe, expect, it } from 'vitest'
import { defaultConnectionInput, defaultTaskSpec } from '@/utils/defaults'

describe('form defaults', () => {
  it('creates independent connection forms without shared nested state', () => {
    const first = defaultConnectionInput()
    const second = defaultConnectionInput()
    first.tls.enabled = true
    first.sentinel.address = 'sentinel.internal:26379'
    expect(second.tls.enabled).toBe(false)
    expect(second.sentinel.address).toBe('127.0.0.1:26379')
  })

  it('uses safe RedisShake defaults for sync and scan tasks', () => {
    const sync = defaultTaskSpec('sync')
    const scan = defaultTaskSpec('scan')
    expect(sync.sync_reader).toMatchObject({ sync_rdb: true, sync_aof: true })
    expect(sync.advanced.empty_db_before_sync).toBe(false)
    expect(scan.scan_reader).toMatchObject({ scan: true, count: 1 })
    expect(scan.filter.allow_keys).toEqual([])
  })
})
