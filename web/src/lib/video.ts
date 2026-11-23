// Demo-video URL parsing for the exercise cues sheet. Pure so the URL rules
// are unit-tested. Only YouTube and Vimeo are embedded (the CSP frame-src
// allows just those players); anything else is shown as a plain link.

export type DemoVideo =
  | { kind: 'youtube' | 'vimeo'; embedUrl: string; url: string }
  | { kind: 'link'; url: string }

const YT_ID = /^[A-Za-z0-9_-]{11}$/

function youtubeId(u: URL): string | null {
  const host = u.hostname.replace(/^(www\.|m\.|music\.)/, '')
  if (host === 'youtu.be') return u.pathname.slice(1).split('/')[0] || null
  if (host !== 'youtube.com' && host !== 'youtube-nocookie.com') return null
  if (u.pathname === '/watch') return u.searchParams.get('v')
  const m = u.pathname.match(/^\/(embed|shorts|live|v)\/([^/]+)/)
  return m ? m[2] : null
}

function vimeoId(u: URL): string | null {
  const host = u.hostname.replace(/^www\./, '')
  if (host === 'vimeo.com') return u.pathname.match(/^\/(\d+)/)?.[1] ?? null
  if (host === 'player.vimeo.com') return u.pathname.match(/^\/video\/(\d+)/)?.[1] ?? null
  return null
}

/** Parse a demo URL into an embeddable player or a link. Non-http(s) URLs are dropped. */
export function parseDemoVideo(raw: string | null | undefined): DemoVideo | null {
  const s = (raw ?? '').trim()
  if (!s) return null
  let u: URL
  try {
    u = new URL(s)
  } catch {
    return null
  }
  if (u.protocol !== 'https:' && u.protocol !== 'http:') return null
  const yt = youtubeId(u)
  if (yt && YT_ID.test(yt)) {
    return { kind: 'youtube', embedUrl: `https://www.youtube-nocookie.com/embed/${yt}`, url: u.href }
  }
  const vm = vimeoId(u)
  if (vm) return { kind: 'vimeo', embedUrl: `https://player.vimeo.com/video/${vm}`, url: u.href }
  return { kind: 'link', url: u.href }
}
