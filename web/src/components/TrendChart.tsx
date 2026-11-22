import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import { formatFriendlyDate } from '@/lib/dates'
import { niceTicks, nearestIndex, scale, type TrendPoint } from '@/lib/trend'
import { formatWeight } from '@/lib/utils'

const HEIGHT = 200
const PAD = { top: 12, right: 12, bottom: 24, left: 40 }

/** Width of an element, tracked with ResizeObserver where it exists. */
function useWidth<T extends HTMLElement>(fallback: number) {
  const ref = useRef<T>(null)
  const [width, setWidth] = useState(fallback)
  useEffect(() => {
    const el = ref.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(entries => {
      const w = entries[0]?.contentRect.width
      if (w && w > 0) setWidth(Math.round(w))
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  return [ref, width] as const
}

function shortDate(iso: string): string {
  return formatFriendlyDate(iso, { absolute: true, layout: 'Jan 2, 2006' }).replace(/, \d{4}$/, '')
}

/**
 * Single-series trend line over time (x is proportional to the date).
 * Hover, touch or arrow keys move a crosshair and tooltip; PR sessions get
 * a ringed marker. One series, so the card title names it — no legend.
 */
export function TrendChart({
  points,
  label,
  unit = 'lb',
}: {
  points: TrendPoint[]
  /** Series name, e.g. "Estimated 1RM". */
  label: string
  unit?: string
}) {
  const [wrapRef, width] = useWidth<HTMLDivElement>(600)
  const [active, setActive] = useState<number | null>(null)

  const innerW = Math.max(width - PAD.left - PAD.right, 40)
  const innerH = HEIGHT - PAD.top - PAD.bottom
  const values = points.map(p => p.value)
  const ticks = niceTicks(Math.min(...values), Math.max(...values))
  const y = scale(ticks[0], ticks[ticks.length - 1], PAD.top + innerH, PAD.top)
  const x = scale(points[0].day, points[points.length - 1].day, PAD.left, PAD.left + innerW)
  const xs = points.map(p => x(p.day))
  const line = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${xs[i].toFixed(1)} ${y(p.value).toFixed(1)}`).join(' ')

  // First, middle and last dates on the x axis, deduplicated.
  const xLabelIdx = [...new Set([0, Math.floor((points.length - 1) / 2), points.length - 1])]
  const cur = active != null ? points[active] : null

  function onPointer(e: PointerEvent<SVGRectElement>) {
    const box = e.currentTarget.ownerSVGElement?.getBoundingClientRect()
    if (!box) return
    setActive(nearestIndex(xs, e.clientX - box.left))
  }

  function onKey(e: KeyboardEvent<SVGSVGElement>) {
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault()
      const dir = e.key === 'ArrowRight' ? 1 : -1
      setActive(i => Math.min(Math.max((i ?? (dir > 0 ? -1 : points.length)) + dir, 0), points.length - 1))
    } else if (e.key === 'Escape') {
      setActive(null)
    }
  }

  const summary = `${label} over ${points.length} sessions, from ${formatWeight(Math.round(points[0].value))} to ${formatWeight(Math.round(points[points.length - 1].value))} ${unit}. Use the left and right arrow keys to read each session.`

  return (
    <div ref={wrapRef} className="relative w-full">
      <svg
        width={width}
        height={HEIGHT}
        viewBox={`0 0 ${width} ${HEIGHT}`}
        role="img"
        aria-label={summary}
        tabIndex={0}
        onKeyDown={onKey}
        onBlur={() => setActive(null)}
        className="block touch-pan-y rounded-md text-primary focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        {/* Recessive grid and y labels */}
        {ticks.map(t => (
          <g key={t}>
            <line x1={PAD.left} x2={PAD.left + innerW} y1={y(t)} y2={y(t)} className="stroke-border" strokeWidth={1} />
            <text x={PAD.left - 6} y={y(t)} dy="0.32em" textAnchor="end" className="fill-muted-foreground text-[11px] tabular-nums">
              {formatWeight(t)}
            </text>
          </g>
        ))}
        {xLabelIdx.map(i => (
          <text
            key={i}
            x={xs[i]}
            y={HEIGHT - 6}
            textAnchor={i === 0 && points.length > 1 ? 'start' : i === points.length - 1 && points.length > 1 ? 'end' : 'middle'}
            className="fill-muted-foreground text-[11px]"
          >
            {shortDate(points[i].date)}
          </text>
        ))}

        <path d={line} fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
        {points.map((p, i) => (
          p.pr || points.length <= 24 ? (
            <circle
              key={p.date + i}
              cx={xs[i]}
              cy={y(p.value)}
              r={p.pr ? 4.5 : 3}
              className={p.pr ? 'fill-background' : 'fill-current'}
              stroke="currentColor"
              strokeWidth={p.pr ? 2 : 0}
            />
          ) : null
        ))}

        {cur && (
          <g aria-hidden="true">
            <line x1={xs[active!]} x2={xs[active!]} y1={PAD.top} y2={PAD.top + innerH} className="stroke-muted-foreground" strokeWidth={1} strokeDasharray="3 3" />
            <circle cx={xs[active!]} cy={y(cur.value)} r={5} className="fill-current stroke-background" strokeWidth={2} />
          </g>
        )}

        {/* Hit target larger than the marks */}
        <rect
          x={PAD.left - 8}
          y={0}
          width={innerW + 16}
          height={HEIGHT}
          fill="transparent"
          onPointerMove={onPointer}
          onPointerDown={onPointer}
          onPointerLeave={() => setActive(null)}
        />
      </svg>

      {cur && (
        <div
          role="status"
          className="pointer-events-none absolute top-0 z-10 -translate-x-1/2 rounded-md border border-border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-sm"
          style={{ left: Math.min(Math.max(xs[active!], 70), width - 70) }}
        >
          <p className="font-medium">{shortDate(cur.date)}{cur.pr && <span className="ml-1 text-success-strong">PR</span>}</p>
          <p className="tabular-nums">
            {label}: <span className="font-semibold">{formatWeight(Math.round(cur.value * 10) / 10)} {unit}</span>
          </p>
          {cur.weight != null && (
            <p className="text-muted-foreground tabular-nums">Top set {formatWeight(cur.weight)} × {cur.reps}</p>
          )}
        </div>
      )}
    </div>
  )
}
