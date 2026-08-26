import { describe, expect, it } from '@jest/globals'
import { render, screen } from '@testing-library/react'

import StatusPill from '@/components/StatusPill'

describe('StatusPill', () => {
  it('renders semantic label and active motion class', () => {
    const { container } = render(<StatusPill label="运行中" tone="active" pulse />)
    expect(screen.getByText('运行中')).toBeInTheDocument()
    expect(container.firstElementChild).toHaveClass('tone-active', 'is-pulsing')
  })
})
