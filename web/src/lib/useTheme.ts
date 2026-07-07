import { useState, useEffect } from 'react'

type Theme = 'dark' | 'light'

/**
 * Resolve the initial theme: an explicit stored choice wins; otherwise fall
 * back to the OS preference via prefers-color-scheme (previously hard-defaulted
 * to dark and ignored the media query).
 */
function getInitialTheme(): Theme {
  const stored = localStorage.getItem('theme')
  if (stored === 'light' || stored === 'dark') return stored
  if (typeof window !== 'undefined' && window.matchMedia) {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }
  return 'dark'
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(getInitialTheme)

  // Reflect the current theme onto <html>. Persistence happens only on an
  // explicit toggle so that, until the user chooses, we keep tracking the OS.
  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
  }, [theme])

  // While the user hasn't made an explicit choice, follow OS changes live.
  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = (e: MediaQueryListEvent) => {
      if (localStorage.getItem('theme')) return
      setThemeState(e.matches ? 'dark' : 'light')
    }
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [])

  function toggleTheme() {
    setThemeState(prev => {
      const next = prev === 'dark' ? 'light' : 'dark'
      localStorage.setItem('theme', next)
      return next
    })
  }

  return { theme, toggleTheme }
}
