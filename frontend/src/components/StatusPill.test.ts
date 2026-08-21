import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StatusPill from '@/components/StatusPill.vue'

describe('StatusPill', () => {
  it('renders semantic label and active motion class', () => {
    const wrapper = mount(StatusPill, { props: { label: '运行中', tone: 'active', pulse: true } })
    expect(wrapper.text()).toContain('运行中')
    expect(wrapper.classes()).toContain('tone-active')
    expect(wrapper.classes()).toContain('is-pulsing')
  })
})
