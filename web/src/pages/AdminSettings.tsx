import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { SettingCategoryData, SettingValueData } from '@/api/types'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Table, TableBody, TableCell, TableRow } from '@/components/ui/table'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Alert } from '@/components/ui/alert'

export function AdminSettings() {
  const { data: categories, isLoading, error } = useQuery({
    queryKey: ['admin-settings'],
    queryFn: () => api.listSettings(),
  })

  if (isLoading) return <Spinner />
  if (error) return <p className="text-destructive">Failed to load settings.</p>

  return (
    <div className="max-w-3xl">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      <div className="space-y-6">
        {categories?.map((cat: SettingCategoryData) => (
          <SettingCategoryCard key={cat.category} category={cat} />
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

function SettingCategoryCard({ category }: { category: SettingCategoryData }) {
  const queryClient = useQueryClient()
  const [drafts, setDrafts] = useState<Record<string, string>>(() => {
    const initial: Record<string, string> = {}
    for (const s of category.settings) {
      if (!s.read_only) initial[s.key] = s.value
    }
    return initial
  })
  const [status, setStatus] = useState<'idle' | 'saving' | 'saved'>('idle')

  const mutation = useMutation({
    mutationFn: async (changes: { key: string; value: string }[]) => {
      for (const c of changes) {
        await api.updateSetting(c.key, c.value)
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-settings'] })
      setStatus('saved')
      setTimeout(() => setStatus('idle'), 2000)
    },
  })

  const hasChanges = category.settings.some(
    s => !s.read_only && drafts[s.key] !== s.value
  )

  function handleSave() {
    const changes = category.settings
      .filter(s => !s.read_only && drafts[s.key] !== s.value)
      .map(s => ({ key: s.key, value: drafts[s.key] }))
    if (changes.length > 0) mutation.mutate(changes)
  }

  function handleReset() {
    const reset: Record<string, string> = {}
    for (const s of category.settings) {
      if (!s.read_only) reset[s.key] = s.value
    }
    setDrafts(reset)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{category.category}</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableBody>
            {category.settings.map(setting => (
              <SettingRow
                key={setting.key}
                setting={setting}
                draft={drafts[setting.key]}
                onDraftChange={val => setDrafts(d => ({ ...d, [setting.key]: val }))}
              />
            ))}
          </TableBody>
        </Table>
      </CardContent>
      {category.settings.some(s => !s.read_only) && (
        <CardFooter className="flex justify-end gap-2 border-t pt-4">
          {hasChanges && (
            <Button variant="ghost" size="sm" onClick={handleReset}>Reset</Button>
          )}
          <Button
            size="sm"
            onClick={handleSave}
            disabled={!hasChanges && status !== 'saved'}
          >
            {status === 'saving' ? 'Saving...' : status === 'saved' ? 'Saved ✓' : 'Save'}
          </Button>
        </CardFooter>
      )}
    </Card>
  )
}

function SettingRow({ setting, draft, onDraftChange }: {
  setting: SettingValueData
  draft?: string
  onDraftChange: (val: string) => void
}) {
  const label = setting.key.split('.').slice(1).join(' ').replace(/_/g, ' ')

  function renderControl() {
    const value = draft ?? setting.value

    if (setting.field_type === 'select' && setting.options?.length) {
      return (
        <Select value={value} onValueChange={(v) => onDraftChange(v ?? '')}>
          <SelectTrigger className="font-mono">
            <SelectValue>{(val) => val || <span className="text-muted-foreground italic">Select...</span>}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            {setting.options.map(opt => (
              <SelectItem key={opt} value={opt}>{opt || '(none)'}</SelectItem>
            ))}
          </SelectContent>
        </Select>
      )
    }

    if (setting.field_type === 'textarea') {
      return (
        <Textarea
          value={value}
          onChange={e => onDraftChange(e.target.value)}
          className="font-mono"
          placeholder={setting.description || `Enter ${label}`}
          rows={3}
        />
      )
    }

    if (setting.field_type === 'number') {
      return (
        <Input
          type="number"
          value={value}
          onChange={e => onDraftChange(e.target.value)}
          className="font-mono"
          placeholder={setting.description || `Enter ${label}`}
        />
      )
    }

    return (
      <Input
        type={setting.field_type === 'password' ? 'password' : 'text'}
        value={value}
        onChange={e => onDraftChange(e.target.value)}
        className="font-mono"
        placeholder={setting.description || `Enter ${label}`}
      />
    )
  }

  return (
    <TableRow>
      <TableCell className="font-medium capitalize whitespace-normal w-1/3 align-top">
        <div>{label}</div>
        {setting.description && (
          <div className="text-xs text-muted-foreground font-normal normal-case mt-0.5">{setting.description}</div>
        )}
      </TableCell>
      <TableCell>
        {setting.read_only ? (
          <div className="flex items-center gap-2">
            <span className="font-mono text-muted-foreground">{setting.masked || setting.value || <span className="italic">(empty)</span>}</span>
            {setting.source !== 'default' && (
              <Badge variant={setting.source === 'env' ? 'default' : 'secondary'}>{setting.source}</Badge>
            )}
          </div>
        ) : (
          <div className="flex items-center gap-2">
            {renderControl()}
            {setting.source !== 'default' && (
              <Badge variant={setting.source === 'env' ? 'default' : 'secondary'}>{setting.source}</Badge>
            )}
          </div>
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