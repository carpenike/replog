import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

export function PreferencesPage() {
  const queryClient = useQueryClient()

  const { data: prefs, isLoading } = useQuery({
    queryKey: ['preferences'],
    queryFn: () => api.getPreferences(),
  })

  const [weightUnit, setWeightUnit] = useState('')
  const [timezone, setTimezone] = useState('')
  const [dateFormat, setDateFormat] = useState('')
  const [saved, setSaved] = useState(false)

  // Initialize form when data loads
  if (prefs && !weightUnit && !timezone) {
    setWeightUnit(prefs.weight_unit)
    setTimezone(prefs.timezone)
    setDateFormat(prefs.date_format)
  }

  const mutation = useMutation({
    mutationFn: () => api.updatePreferences({ weight_unit: weightUnit, timezone, date_format: dateFormat }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['preferences'] })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    },
  })

  if (isLoading) return <Spinner />

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold mb-6">Preferences</h1>

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {saved && (
          <div className="rounded-md bg-success/10 border border-success/30 p-3 text-sm text-success">
            Preferences saved
          </div>
        )}

        <div>
          <label htmlFor="weightUnit" className="block text-sm font-medium mb-1">Weight Unit</label>
          <select id="weightUnit" value={weightUnit} onChange={e => setWeightUnit(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
            <option value="lbs">Pounds (lbs)</option>
            <option value="kg">Kilograms (kg)</option>
          </select>
        </div>

        <div>
          <label htmlFor="timezone" className="block text-sm font-medium mb-1">Timezone</label>
          <select id="timezone" value={timezone} onChange={e => setTimezone(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
            <option value="America/New_York">Eastern (New York)</option>
            <option value="America/Chicago">Central (Chicago)</option>
            <option value="America/Denver">Mountain (Denver)</option>
            <option value="America/Los_Angeles">Pacific (Los Angeles)</option>
            <option value="UTC">UTC</option>
            <option value="Europe/London">Europe/London</option>
          </select>
        </div>

        <div>
          <label htmlFor="dateFormat" className="block text-sm font-medium mb-1">Date Format</label>
          <select id="dateFormat" value={dateFormat} onChange={e => setDateFormat(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
            <option value="Jan 2, 2006">Jan 2, 2006</option>
            <option value="2006-01-02">2006-01-02</option>
            <option value="01/02/2006">01/02/2006</option>
            <option value="02/01/2006">02/01/2006</option>
          </select>
        </div>

        <button type="submit" disabled={mutation.isPending}
          className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
          {mutation.isPending ? 'Saving...' : 'Save Preferences'}
        </button>
      </form>
    </div>
  )
}
