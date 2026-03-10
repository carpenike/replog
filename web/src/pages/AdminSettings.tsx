import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { SettingCategoryData } from '@/api/types'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Alert } from '@/components/ui/alert'

export function AdminSettings() {
  const queryClient = useQueryClient()
  const [saved, setSaved] = useState<string | null>(null)

  const { data: categories, isLoading, error } = useQuery({
    queryKey: ['admin-settings'],
    queryFn: () => api.listSettings(),
  })

  const mutation = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) => api.updateSetting(key, value),
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ['admin-settings'] })
      setSaved(vars.key)
      setTimeout(() => setSaved(null), 2000)
    },
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load settings.</p>

  return (
    <div className="max-w-3xl">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      <div className="space-y-6">
        {categories?.map((cat: SettingCategoryData) => (
          <Card key={cat.category}>
            <CardHeader>
              <CardTitle>{cat.category}</CardTitle>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Setting</TableHead>
                    <TableHead>Value</TableHead>
                    <TableHead>Source</TableHead>
                    <TableHead className="w-20"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {cat.settings.map(setting => (
                    <SettingRow
                      key={setting.key}
                      settingKey={setting.key}
                      value={setting.value}
                      masked={setting.masked}
                      source={setting.source}
                      readOnly={setting.read_only}
                      isSaved={saved === setting.key}
                      onSave={(value) => mutation.mutate({ key: setting.key, value })}
                    />
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        ))}

        {/* Test Connections */}
        <Card>
          <CardHeader>
            <CardTitle>Test Connections</CardTitle>
            <CardDescription>Verify that external services are reachable.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex flex-wrap gap-3">
              <TestButton label="Test LLM" onClick={() => api.testLLMConnection()} />
              <TestButton label="Test Notifications" onClick={() => api.testNotifyConnection()} />
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

function SettingRow({ settingKey, value, masked, source, readOnly, isSaved, onSave }: {
  settingKey: string
  value: string
  masked: string
  source: string
  readOnly: boolean
  isSaved: boolean
  onSave: (value: string) => void
}) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(value)
  const label = settingKey.split('.').slice(1).join(' ').replace(/_/g, ' ')

  if (editing) {
    return (
      <TableRow>
        <TableCell className="font-medium capitalize whitespace-normal">{label}</TableCell>
        <TableCell colSpan={2}>
          <div className="flex gap-2">
            <Input
              type="text"
              value={draft}
              onChange={e => setDraft(e.target.value)}
              className="font-mono"
              onKeyDown={e => { if (e.key === 'Enter') { onSave(draft); setEditing(false) } if (e.key === 'Escape') { setEditing(false); setDraft(value) } }}
              autoFocus
            />
          </div>
        </TableCell>
        <TableCell>
          <div className="flex gap-1">
            <Button size="xs" onClick={() => { onSave(draft); setEditing(false) }}>Save</Button>
            <Button size="xs" variant="ghost" onClick={() => { setEditing(false); setDraft(value) }}>Cancel</Button>
          </div>
        </TableCell>
      </TableRow>
    )
  }

  return (
    <TableRow>
      <TableCell className="font-medium capitalize whitespace-normal">{label}</TableCell>
      <TableCell className="font-mono text-muted-foreground whitespace-normal">
        {isSaved ? <span className="text-success">Saved ✓</span> : (masked || value || <span className="italic">(empty)</span>)}
      </TableCell>
      <TableCell>
        {source !== 'default' && (
          <Badge variant={source === 'env' ? 'default' : 'secondary'}>{source}</Badge>
        )}
      </TableCell>
      <TableCell>
        {!readOnly && (
          <Button variant="ghost" size="xs" onClick={() => setEditing(true)}>Edit</Button>
        )}
      </TableCell>
    </TableRow>
  )
}

function TestButton({ label, onClick }: { label: string; onClick: () => Promise<{ success: boolean; error?: string }> }) {
  const [status, setStatus] = useState<'idle' | 'testing' | 'success' | 'error'>('idle')
  const [errorMsg, setErrorMsg] = useState('')

  async function handleTest() {
    setStatus('testing')
    setErrorMsg('')
    try {
      const result = await onClick()
      setStatus(result.success ? 'success' : 'error')
      if (!result.success && result.error) setErrorMsg(result.error)
    } catch {
      setStatus('error')
      setErrorMsg('Connection failed')
    }
    setTimeout(() => setStatus('idle'), 5000)
  }

  return (
    <div>
      <Button
        variant={status === 'success' ? 'default' : status === 'error' ? 'destructive' : 'outline'}
        onClick={handleTest}
        disabled={status === 'testing'}
      >
        {status === 'testing' ? 'Testing...' :
         status === 'success' ? '✓ Connected' :
         status === 'error' ? '✗ Failed' :
         label}
      </Button>
      {errorMsg && <Alert variant="destructive" className="mt-2">{errorMsg}</Alert>}
    </div>
  )
}