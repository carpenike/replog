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
