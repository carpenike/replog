import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { PlateCounter, type PlateCounterState } from '@/components/PlateCounter'
import { Card, CardContent } from '@/components/ui/card'
import { inventoryFromSetup, KG_COMPETITION, normalizeUnit, type PlateInventory } from '@/lib/plates'

const STORAGE_KEY = 'replog.plateCounter'

function loadState(): PlateCounterState | null {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    const v = JSON.parse(raw)
    if (Array.isArray(v?.stack) && typeof v?.useCollars === 'boolean') return { stack: v.stack.map(String), useCollars: v.useCollars }
  } catch {
    // Storage blocked or bad JSON: start with an empty bar.
  }
  return null
}

function saveState(s: PlateCounterState) {
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify(s))
  } catch {
    // Not worth surfacing; the counter still works for this view.
  }
}

/**
 * Standalone plate calculator: tap the plates loaded on one side of the bar
 * and read the total in lb and kg. Uses the signed-in athlete's plate setup,
 * or the default kg competition setup when there isn't one.
 */
export function PlateCalculatorPage() {
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: () => api.me() })
  const { data: prefs } = useQuery({
    queryKey: ['preferences'],
    queryFn: () => api.getPreferences(),
    staleTime: 10 * 60_000,
  })
  const athleteId = me?.athlete_id ?? undefined
  const { data: plateSetup } = useQuery({
    queryKey: ['plate-setup', athleteId],
    queryFn: () => api.getPlateSetup(athleteId!),
    enabled: athleteId != null,
    staleTime: 10 * 60_000,
  })
  const inventory: PlateInventory = useMemo(
    () => (plateSetup ? inventoryFromSetup(plateSetup) : KG_COMPETITION),
    [plateSetup],
  )
  const [state, setState] = useState<PlateCounterState | null>(loadState)
  const current = state ?? { stack: [], useCollars: inventory.useCollars }

  function update(next: PlateCounterState) {
    setState(next)
    saveState(next)
  }

  return (
    <div className="mx-auto max-w-md space-y-4">
      <PageHeader
        title="Plate calculator"
        description="Tap the plates on one side of the bar to see the total."
      />
      <Card>
        <CardContent>
          <PlateCounter
            inventory={inventory}
            unit={normalizeUnit(prefs?.weight_unit)}
            state={current}
            onChange={update}
          />
        </CardContent>
      </Card>
    </div>
  )
}
