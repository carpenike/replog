import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'

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
          <Label htmlFor="weightUnit" >Weight Unit</Label>
          <select id="weightUnit" value={weightUnit} onChange={e => setWeightUnit(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
            <option value="lbs">Pounds (lbs)</option>
            <option value="kg">Kilograms (kg)</option>
          </select>
        </div>

        <div>
          <Label htmlFor="timezone" >Timezone</Label>
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
          <Label htmlFor="dateFormat" >Date Format</Label>
          <select id="dateFormat" value={dateFormat} onChange={e => setDateFormat(e.target.value)}
            className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
            <option value="Jan 2, 2006">Jan 2, 2006</option>
            <option value="2006-01-02">2006-01-02</option>
            <option value="01/02/2006">01/02/2006</option>
            <option value="02/01/2006">02/01/2006</option>
          </select>
        </div>

        <Button type="submit" disabled={mutation.isPending}
          >
          {mutation.isPending ? 'Saving...' : 'Save Preferences'}
        </Button>
      </form>

      {/* Passkey Management */}
      <Card className="mt-8">
        <CardContent>
        <h2 className="text-lg font-semibold mb-2">Passkeys</h2>
        <p className="text-sm text-muted-foreground mb-4">
          Manage your passkey credentials for passwordless login. Passkeys use your device's biometric or PIN to sign in securely.
        </p>
        <a href="/preferences">
          <Button>Manage Passkeys</Button>
        </a>
        </CardContent>
      </Card>
    </div>
  )
}
