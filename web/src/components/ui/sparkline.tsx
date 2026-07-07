import { cn } from '@/lib/utils'

/**
 * Lightweight inline-SVG sparkline — no chart library, no runtime deps.
 * Values are plotted left→right; the line uses currentColor so callers control
 * the hue via text-* utilities. An optional area fill sits under the line.
 */
export function Sparkline({
  data,
  width = 120,
  height = 32,
  strokeWidth = 1.5,
  fill = true,
  className,
  ariaLabel,
}: {
  data: number[]
  width?: number
  height?: number
  strokeWidth?: number
  fill?: boolean
  className?: string
  ariaLabel?: string
}) {
  const points = data.filter(v => Number.isFinite(v))
  if (points.length < 2) return null

  const min = Math.min(...points)
  const max = Math.max(...points)
  const range = max - min || 1
  const pad = strokeWidth
  const w = width - pad * 2
  const h = height - pad * 2

  const coords = points.map((v, i) => {
    const x = pad + (i / (points.length - 1)) * w
    const y = pad + (1 - (v - min) / range) * h
    return [x, y] as const
  })

  const line = coords.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`).join(' ')
  const area = `${line} L${coords[coords.length - 1][0].toFixed(2)} ${height} L${coords[0][0].toFixed(2)} ${height} Z`

  return (
    <svg
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
      className={cn('overflow-visible text-primary', className)}
      role="img"
      aria-label={ariaLabel}
      preserveAspectRatio="none"
    >
      {fill && <path d={area} fill="currentColor" fillOpacity={0.12} stroke="none" />}
      <path d={line} fill="none" stroke="currentColor" strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  )
}
