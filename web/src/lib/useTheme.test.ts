import { describe, expect, it } from 'vitest'
import { applyThemeColor, THEME_COLORS } from '@/lib/useTheme'

describe('applyThemeColor', () => {
  it('updates the existing theme-color meta to match the theme', () => {
    const doc = document.implementation.createHTMLDocument('t')
    const meta = doc.createElement('meta')
    meta.name = 'theme-color'
    meta.content = '#000000'
    doc.head.appendChild(meta)

    applyThemeColor('light', doc)
    expect(meta.content).toBe(THEME_COLORS.light)
    applyThemeColor('dark', doc)
    expect(meta.content).toBe(THEME_COLORS.dark)
  })

  it('creates the meta when it is missing', () => {
    const doc = document.implementation.createHTMLDocument('t')
    applyThemeColor('light', doc)
    expect(doc.querySelector('meta[name="theme-color"]')?.getAttribute('content')).toBe('#ffffff')
  })
})
