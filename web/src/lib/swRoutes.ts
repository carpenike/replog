// Route predicates for the service worker (pwa-sw.ts), kept here so they
// are unit-tested.

const SET_VIDEO = /^\/api\/athletes\/\d+\/workouts\/\d+\/sets\/\d+\/video(\/|$)/

/**
 * Form-check video routes. They bypass the service worker entirely: a clip
 * must not land in the API read cache (it can be 100 MB), Range requests
 * from the video element go straight to the network, and an upload that
 * fails offline must not be queued and replayed later.
 */
export function isSetVideoPath(pathname: string): boolean {
  return SET_VIDEO.test(pathname)
}

const SHARE_PAGE = /^\/s\//
const SHARE_API = /^\/api\/share\//

/**
 * Public share-link routes: the /s/{token} viewer page and /api/share/...
 * The service worker never serves or caches them. The page must come from
 * the network so the server's no-store, noindex and CSP headers (and its 404
 * for a dead link) apply, and nothing about a shared clip is left in a cache.
 */
export function isSharePath(pathname: string): boolean {
  return SHARE_PAGE.test(pathname) || SHARE_API.test(pathname)
}
