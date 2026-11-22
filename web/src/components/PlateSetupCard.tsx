import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Plus, RotateCcw, Trash2 } from 'lucide-react'
import { api, ApiError } from '@/api/client'
import type { PlateSetup, PlateSetupPreset } from '@/api/schemas'
import { Field } from '@/components/Field'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { emptyBarKg, fromKg, maxLoadable, normalizeUnit, round1, solvePlates, formatPerSide, type PlateInventory, type WeightUnit } from '@/lib/plates'
import { useConfirm } from '@/lib/useConfirm'

interface PlateRow {
  key: number
  weight: string
  unit: WeightUnit
  pairs: string
}

interface FormState {
  barWeight: string
  barUnit: WeightUnit
  collarWeight: string
  collarUnit: WeightUnit
  useCollars: boolean
  plates: PlateRow[]
}

let rowKey = 0

function formFromSetup(s: PlateSetup): FormState {
  return {
    barWeight: String(s.bar?.weight ?? 20),
    barUnit: normalizeUnit(s.bar?.unit),
    collarWeight: String(s.collar?.weight ?? 0),
    collarUnit: normalizeUnit(s.collar?.unit),
    useCollars: s.use_collars ?? false,
    plates: (s.plates ?? []).map(p => ({
      key: ++rowKey,
      weight: String(p.weight ?? ''),
      unit: normalizeUnit(p.unit),
      pairs: String(p.pairs ?? 1),
    })),
  }
}

function num(s: string): number {
  const v = parseFloat(s)
  return isNaN(v) ? 0 : v
}

function inventoryFromForm(f: FormState): PlateInventory {
  return {
    bar: { weight: num(f.barWeight), unit: f.barUnit },
    collar: { weight: num(f.collarWeight), unit: f.collarUnit },
    useCollars: f.useCollars,
    plates: f.plates
      .map(p => ({ weight: num(p.weight), unit: p.unit, pairs: Math.max(0, Math.floor(num(p.pairs))) }))
      .filter(p => p.weight > 0),
  }
}

/** Client-side check; the server validates again and its message wins. */
function formError(f: FormState): string | null {
  if (!(num(f.barWeight) >= 0 && num(f.barWeight) <= 100)) return 'Bar weight must be between 0 and 100.'
  if (!(num(f.collarWeight) >= 0 && num(f.collarWeight) <= 25)) return 'Collar weight must be between 0 and 25.'
  const seen = new Set<string>()
  for (const p of f.plates) {
    const w = num(p.weight)
    if (!(w > 0 && w <= 100)) return 'Each plate needs a weight above 0 and at most 100.'
    const pairs = Number(p.pairs)
    if (!Number.isInteger(pairs) || pairs < 0 || pairs > 50) return 'Pairs must be a whole number from 0 to 50.'
    const k = `${w}${p.unit}`
    if (seen.has(k)) return `${w} ${p.unit === 'kg' ? 'kg' : 'lb'} is listed twice.`
    seen.add(k)
  }
  return null
}

function UnitSelect({ id, value, onChange, label }: { id: string; value: WeightUnit; onChange: (u: WeightUnit) => void; label: string }) {
  return (
    <Select value={value} onValueChange={v => onChange(normalizeUnit(v))}>
      <SelectTrigger id={id} aria-label={label} className="h-11 w-20 md:h-8">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="kg">kg</SelectItem>
        <SelectItem value="lbs">lb</SelectItem>
      </SelectContent>
    </Select>
  )
}

interface AthleteOption {
  id: number
  name: string
}

interface EditorProps {
  athleteId: number
  setup: PlateSetup
  presets: PlateSetupPreset[]
  loggingUnit: WeightUnit
  /** Other athletes whose setup can be copied in (same home gym). */
  copyFrom: AthleteOption[]
}

function PlateSetupEditor({ athleteId, setup, presets, loggingUnit, copyFrom }: EditorProps) {
  const queryClient = useQueryClient()
  const { confirm, dialog } = useConfirm()
  const [form, setForm] = useState<FormState>(() => formFromSetup(setup))
  const inv = inventoryFromForm(form)
  const error = formError(form)
  const unitLabel = loggingUnit === 'kg' ? 'kg' : 'lb'
  const example = loggingUnit === 'kg' ? 100 : 225
  const exampleSolution = solvePlates(example, inv, loggingUnit)

  const save = useMutation({
    mutationFn: () => api.updatePlateSetup(athleteId, {
      bar: { weight: num(form.barWeight), unit: form.barUnit },
      collar: { weight: num(form.collarWeight), unit: form.collarUnit },
      use_collars: form.useCollars,
      plates: form.plates.map(p => ({ weight: num(p.weight), unit: p.unit, pairs: Number(p.pairs) })),
    }),
    meta: { skipGlobalError: true },
    onSuccess: (saved) => {
      queryClient.setQueryData(['plate-setup', athleteId], saved)
      toast.success('Plate setup saved')
    },
    onError: (err) => toast.error(err instanceof ApiError ? err.message : 'Failed to save plate setup'),
  })

  const reset = useMutation({
    mutationFn: () => api.resetPlateSetup(athleteId),
    meta: { skipGlobalError: true },
    onSuccess: (def) => {
      queryClient.setQueryData(['plate-setup', athleteId], def)
      setForm(formFromSetup(def))
      toast.success('Plate setup reset to the default')
    },
    onError: () => toast.error('Failed to reset plate setup'),
  })

  async function copySetupFrom(otherId: number) {
    try {
      const other = await queryClient.fetchQuery({
        queryKey: ['plate-setup', otherId],
        queryFn: () => api.getPlateSetup(otherId),
      })
      setForm(formFromSetup(other))
      toast.info('Copied. Save to keep it.')
    } catch {
      toast.error('Failed to load that setup')
    }
  }

  function updatePlate(key: number, patch: Partial<PlateRow>) {
    setForm(f => ({ ...f, plates: f.plates.map(p => p.key === key ? { ...p, ...patch } : p) }))
  }

  return (
    <form
      className="space-y-4"
      onSubmit={e => {
        e.preventDefault()
        if (error) { toast.error(error); return }
        save.mutate()
      }}
    >
      {presets.length > 0 && (
        <div>
          <p className="mb-1.5 text-sm font-medium">Start from a preset</p>
          <div className="flex flex-wrap gap-2">
            {copyFrom.length > 0 && (
              <Select value="" onValueChange={v => { if (v) void copySetupFrom(Number(v)) }}>
                <SelectTrigger aria-label="Copy another athlete's setup" className="h-11 md:h-8">
                  <SelectValue placeholder="Copy from…" />
                </SelectTrigger>
                <SelectContent>
                  {copyFrom.map(o => <SelectItem key={o.id} value={String(o.id)}>{o.name}</SelectItem>)}
                </SelectContent>
              </Select>
            )}
            {presets.map(p => (
              <Button
                key={p.key}
                type="button"
                variant="outline"
                size="sm"
                className="h-11 md:h-8"
                onClick={() => p.setup && setForm(formFromSetup(p.setup))}
              >
                {p.name}
              </Button>
            ))}
          </div>
        </div>
      )}

      <div className="grid gap-3 sm:grid-cols-2">
        <Field label="Bar" id={`bar-${athleteId}`}>
          <div className="flex gap-2">
            <Input id={`bar-${athleteId}`} type="number" inputMode="decimal" step="0.5" min={0} max={100} value={form.barWeight}
              onChange={e => setForm(f => ({ ...f, barWeight: e.target.value }))} />
            <UnitSelect id={`bar-unit-${athleteId}`} label="Bar unit" value={form.barUnit} onChange={u => setForm(f => ({ ...f, barUnit: u }))} />
          </div>
        </Field>
        <Field label="Collar (each)" id={`collar-${athleteId}`}>
          <div className="flex gap-2">
            <Input id={`collar-${athleteId}`} type="number" inputMode="decimal" step="0.5" min={0} max={25} value={form.collarWeight}
              onChange={e => setForm(f => ({ ...f, collarWeight: e.target.value }))} />
            <UnitSelect id={`collar-unit-${athleteId}`} label="Collar unit" value={form.collarUnit} onChange={u => setForm(f => ({ ...f, collarUnit: u }))} />
          </div>
        </Field>
      </div>
      <div className="flex min-h-11 items-center gap-2">
        <Checkbox
          id={`use-collars-${athleteId}`}
          checked={form.useCollars}
          onCheckedChange={checked => setForm(f => ({ ...f, useCollars: checked }))}
        />
        <Label htmlFor={`use-collars-${athleteId}`}>Count collars in the loaded weight</Label>
      </div>

      <fieldset>
        <legend className="mb-1.5 text-sm font-medium">Plates (pairs owned)</legend>
        <ul className="space-y-2">
          {form.plates.map((p, i) => (
            <li key={p.key} className="flex items-center gap-2">
              <Input type="number" inputMode="decimal" step="0.25" min={0} value={p.weight} aria-label={`Plate ${i + 1} weight`}
                onChange={e => updatePlate(p.key, { weight: e.target.value })} className="w-24" />
              <UnitSelect id={`plate-unit-${athleteId}-${p.key}`} label={`Plate ${i + 1} unit`} value={p.unit} onChange={u => updatePlate(p.key, { unit: u })} />
              <span className="text-sm text-muted-foreground" aria-hidden="true">×</span>
              <Input type="number" inputMode="numeric" min={0} max={50} value={p.pairs} aria-label={`Plate ${i + 1} pairs`}
                onChange={e => updatePlate(p.key, { pairs: e.target.value })} className="w-20" />
              <Button type="button" variant="ghost" size="icon-touch" aria-label={`Remove plate ${i + 1}`}
                className="text-muted-foreground hover:text-destructive"
                onClick={() => setForm(f => ({ ...f, plates: f.plates.filter(x => x.key !== p.key) }))}>
                <Trash2 aria-hidden="true" />
              </Button>
            </li>
          ))}
        </ul>
        <Button type="button" variant="outline" size="sm" className="mt-2 h-11 md:h-8"
          onClick={() => setForm(f => ({ ...f, plates: [...f.plates, { key: ++rowKey, weight: '', unit: f.plates.at(-1)?.unit ?? f.barUnit, pairs: '1' }] }))}>
          <Plus aria-hidden="true" /> Add plate
        </Button>
      </fieldset>

      <div className="rounded-lg bg-muted/50 p-3 text-sm" aria-live="polite">
        <p>
          Empty bar {round1(fromKg(emptyBarKg(inv), loggingUnit))} {unitLabel} · heaviest load {maxLoadable(inv, loggingUnit)} {unitLabel}
        </p>
        <p className="text-muted-foreground">
          {example} {unitLabel}: {formatPerSide(exampleSolution.perSide)} per side = {exampleSolution.loaded} {unitLabel}
        </p>
        <p className="mt-1 text-xs text-muted-foreground">You log in {unitLabel} (set in Weight Unit above). Plates keep their own unit.</p>
      </div>

      {error && <p className="text-sm text-destructive" role="alert">{error}</p>}

      <div className="flex flex-wrap gap-2">
        <Button type="submit" disabled={save.isPending || !!error}>
          {save.isPending ? 'Saving…' : 'Save plate setup'}
        </Button>
        {!setup.is_default && (
          <Button
            type="button"
            variant="ghost"
            disabled={reset.isPending}
            onClick={async () => {
              if (await confirm({ title: 'Reset plate setup', description: 'Go back to the default 20 kg bar with competition kg plates?', confirmLabel: 'Reset' }))
                reset.mutate()
            }}
          >
            <RotateCcw aria-hidden="true" /> Reset to default
          </Button>
        )}
      </div>
      {dialog()}
    </form>
  )
}

function PlateSetupLoader({ athleteId, loggingUnit, copyFrom }: { athleteId: number; loggingUnit: WeightUnit; copyFrom: AthleteOption[] }) {
  const setupQuery = useQuery({
    queryKey: ['plate-setup', athleteId],
    queryFn: () => api.getPlateSetup(athleteId),
  })
  const { data: presets } = useQuery({
    queryKey: ['plate-setup-presets'],
    queryFn: () => api.listPlateSetupPresets(),
    staleTime: Infinity,
  })
  if (setupQuery.isLoading) return <Skeleton className="h-64 w-full" />
  if (setupQuery.isError || !setupQuery.data) {
    return (
      <div className="space-y-2">
        <p className="text-sm text-destructive">Couldn't load the plate setup.</p>
        <Button variant="outline" size="sm" onClick={() => setupQuery.refetch()}>Retry</Button>
      </div>
    )
  }
  return (
    <PlateSetupEditor
      key={`${athleteId}-${setupQuery.data.updated_at ?? 'default'}`}
      athleteId={athleteId}
      setup={setupQuery.data}
      presets={presets ?? []}
      loggingUnit={loggingUnit}
      copyFrom={copyFrom}
    />
  )
}

interface PlateSetupCardProps {
  /** The signed-in user's own athlete, when linked. */
  ownAthleteId?: number | null
  isCoach: boolean
  loggingUnit: string
}

/**
 * Preferences → Plate setup: the bar, collars and plate inventory the plate
 * calculator and warm-up ramps load from. Saved per athlete, so a coach can
 * set it up for a kid who trains in the same home gym.
 */
export function PlateSetupCard({ ownAthleteId, isCoach, loggingUnit }: PlateSetupCardProps) {
  const ref = useRef<HTMLDivElement>(null)
  const { data: athletes } = useQuery({
    queryKey: ['athletes'],
    queryFn: () => api.listAthletes(),
    enabled: isCoach,
  })
  const [picked, setPicked] = useState<number | null>(null)
  const athleteId = picked ?? ownAthleteId ?? athletes?.[0]?.id ?? null

  // Deep link from the session's plate calculator (/preferences#plate-setup).
  useEffect(() => {
    if (window.location.hash === '#plate-setup') ref.current?.scrollIntoView({ block: 'start' })
  }, [])

  if (!ownAthleteId && !isCoach) return null
  const options: AthleteOption[] = [
    ...(ownAthleteId && !athletes?.some(a => a.id === ownAthleteId) ? [{ id: ownAthleteId, name: 'Me' }] : []),
    ...(athletes ?? []).map(a => ({ id: a.id, name: a.id === ownAthleteId ? `${a.name} (me)` : a.name })),
  ]

  return (
    <Card className="mt-8 scroll-mt-20" id="plate-setup" ref={ref}>
      <CardHeader>
        <CardTitle>Plate setup</CardTitle>
        <CardDescription>
          Your bar, collars and plates. The session's plate calculator and warm-up ramps load from this.
          Kg plates work fine when you log in lb.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {isCoach && options.length > 1 && (
          <Field label="Athlete" id="plate-setup-athlete" hint="Each athlete keeps their own setup. For kids in the same home gym, use Copy from… and save.">
            <Select value={athleteId != null ? String(athleteId) : ''} onValueChange={v => setPicked(Number(v))}>
              <SelectTrigger id="plate-setup-athlete" className="w-full">
                <SelectValue>{options.find(o => o.id === athleteId)?.name ?? 'Pick an athlete'}</SelectValue>
              </SelectTrigger>
              <SelectContent>
                {options.map(o => <SelectItem key={o.id} value={String(o.id)}>{o.name}</SelectItem>)}
              </SelectContent>
            </Select>
          </Field>
        )}
        {athleteId != null
          ? <PlateSetupLoader key={athleteId} athleteId={athleteId} loggingUnit={normalizeUnit(loggingUnit)} copyFrom={options.filter(o => o.id !== athleteId)} />
          : <p className="text-sm text-muted-foreground">No athletes yet.</p>}
      </CardContent>
    </Card>
  )
}
