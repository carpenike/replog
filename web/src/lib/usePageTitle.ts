import { useEffect } from 'react'

const BASE_TITLE = 'RepLog'

/**
 * Set document.title for the current route, restoring the base title on unmount.
 * Pass a falsy value while data is still loading to keep the base title.
 */
export function usePageTitle(title: string | null | undefined) {
  useEffect(() => {
    document.title = title ? `${title} · ${BASE_TITLE}` : BASE_TITLE
    return () => {
      document.title = BASE_TITLE
    }
  }, [title])
}
