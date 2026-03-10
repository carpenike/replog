import { useState, useEffect } from 'react'

type Theme = 'dark' | 'light'

function getStoredTheme(): Theme {
  const stored = localStorage.getItem('theme')
  if (stored === 'light' || stored === 'dark') return stored
  return 'dark'
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(getStoredTheme)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
    localStorage.setItem('theme', theme)
  }, [theme])

  function toggleTheme() {
    setThemeState(prev => prev === 'dark' ? 'light' : 'dark')
  }

  return { theme, toggleTheme }
}
