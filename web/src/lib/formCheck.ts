// Form-check video helpers (gap 12). The server sniffs the file and is the
// authority on type and size; these checks only catch the obvious cases
// before a large upload starts on gym wifi.

/** Content types the server stores. iPhones record QuickTime (.mov). */
export const FORM_CHECK_TYPES = ['video/mp4', 'video/quicktime', 'video/webm'] as const

/** `accept` attribute for the file input. */
export const FORM_CHECK_ACCEPT = 'video/mp4,video/quicktime,video/webm,.mp4,.m4v,.mov,.webm'

/** Default server cap (REPLOG_VIDEO_MAX_MB, 100 MiB unless changed). */
export const DEFAULT_MAX_CLIP_BYTES = 100 * 1024 * 1024

const EXTENSIONS = ['.mp4', '.m4v', '.mov', '.webm']

/** "4.2 MB", "820 KB". */
export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

/**
 * Returns a message when the file is clearly not an acceptable clip, or
 * null. A file with no reported type (some Android pickers) falls back to
 * its extension.
 */
export function checkClip(file: Pick<File, 'name' | 'type' | 'size'>, maxBytes = DEFAULT_MAX_CLIP_BYTES): string | null {
  if (file.size === 0) return 'That file is empty.'
  const type = file.type.toLowerCase()
  const name = file.name.toLowerCase()
  const typeOk = (FORM_CHECK_TYPES as readonly string[]).includes(type)
  const extOk = EXTENSIONS.some(ext => name.endsWith(ext))
  if (type ? !typeOk : !extOk) return 'Pick an MP4, MOV or WebM video.'
  if (file.size > maxBytes) {
    return `That clip is ${formatBytes(file.size)}; the limit is ${formatBytes(maxBytes)}. Trim it to the set and try again.`
  }
  return null
}
