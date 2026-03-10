import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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
          <Label>Weight Unit</Label>
          <Select value={weightUnit} onValueChange={(val) => setWeightUnit(val ?? "lbs")}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="lbs">Pounds (lbs)</SelectItem>
              <SelectItem value="kg">Kilograms (kg)</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div>
          <Label>Timezone</Label>
          <Select value={timezone} onValueChange={(val) => setTimezone(val ?? "America/New_York")}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="America/New_York">Eastern (New York)</SelectItem>
              <SelectItem value="America/Chicago">Central (Chicago)</SelectItem>
              <SelectItem value="America/Denver">Mountain (Denver)</SelectItem>
              <SelectItem value="America/Los_Angeles">Pacific (Los Angeles)</SelectItem>
              <SelectItem value="UTC">UTC</SelectItem>
              <SelectItem value="Europe/London">Europe/London</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div>
          <Label>Date Format</Label>
          <Select value={dateFormat} onValueChange={(val) => setDateFormat(val ?? "Jan 2, 2006")}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="Jan 2, 2006">Jan 2, 2006</SelectItem>
              <SelectItem value="2006-01-02">2006-01-02</SelectItem>
              <SelectItem value="01/02/2006">01/02/2006</SelectItem>
              <SelectItem value="02/01/2006">02/01/2006</SelectItem>
            </SelectContent>
          </Select>
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
