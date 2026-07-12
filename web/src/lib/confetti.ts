import confetti from 'canvas-confetti'

/**
 * Short celebratory confetti burst. No-op when the user prefers reduced
 * motion — callers keep their toast, only the animation is skipped.
 */
export function celebrate() {
  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  confetti({
    particleCount: 90,
    spread: 70,
    startVelocity: 40,
    origin: { y: 0.8 },
    disableForReducedMotion: true,
  })
}
