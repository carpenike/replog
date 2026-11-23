import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { SettingCategoryData, SettingValueData } from '@/api/types'
import { Check, X } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardFooter, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { Alert } from '@/components/ui/alert'

export function AdminSettings() {
  const query = useQuery({
    queryKey: ['admin-settings'],
    queryFn: () => api.listSettings(),
  })

  return (
    <div className="max-w-3xl">
      <PageHeader title="Settings" />

      <div className="space-y-6">
        <QueryState query={query} resource="settings" skeleton="cards">
          {categories => (
            <>
              {categories.map((cat: SettingCategoryData) => (
                <SettingCategoryCard key={cat.category} category={cat} />
              ))}
            </>
          )}
        </QueryState>

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
        <div className="divide-y divide-border">
            {category.settings.map(setting => (
              <SettingRow
                key={setting.key}
                setting={setting}
                draft={drafts[setting.key]}
                onDraftChange={val => setDrafts(d => ({ ...d, [setting.key]: val }))}
              />
            ))}
        </div>
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
            {status === 'saving' ? 'Saving...' : status === 'saved' ? <><Check aria-hidden="true" /> Saved</> : 'Save'}
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
  const controlId = `setting-${setting.key.replace(/[^a-zA-Z0-9_-]/g, '-')}`
  const descId = setting.description ? `${controlId}-desc` : undefined

  function renderControl() {
    const value = draft ?? setting.value

    if (setting.field_type === 'select' && setting.options?.length) {
      return (
        <Select value={value} onValueChange={(v) => onDraftChange(v ?? '')}>
          <SelectTrigger id={controlId} aria-describedby={descId} className="font-mono">
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
          id={controlId}
          aria-describedby={descId}
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
          id={controlId}
          aria-describedby={descId}
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
        id={controlId}
        aria-describedby={descId}
        type={setting.field_type === 'password' ? 'password' : 'text'}
        value={value}
        onChange={e => onDraftChange(e.target.value)}
        className="font-mono"
        placeholder={setting.description || `Enter ${label}`}
      />
    )
  }

  return (
    <div className="grid gap-2 py-3 md:grid-cols-[1fr_2fr] md:gap-4">
      <div className="text-sm">
        {setting.read_only
          ? <div className="font-medium capitalize">{label}</div>
          : <label htmlFor={controlId} className="font-medium capitalize">{label}</label>}
        {setting.description && (
          <div id={descId} className="text-xs text-muted-foreground font-normal normal-case mt-0.5">{setting.description}</div>
        )}
      </div>
      <div className="min-w-0">
        {setting.read_only ? (
          <div className="flex items-center gap-2">
            <span className="font-mono text-sm text-muted-foreground break-all">{setting.masked || setting.value || <span className="italic">(empty)</span>}</span>
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
      </div>
    </div>
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
         status === 'success' ? <><Check aria-hidden="true" /> Connected</> :
         status === 'error' ? <><X aria-hidden="true" /> Failed</> :
         label}
      </Button>
      {errorMsg && <Alert variant="destructive" className="mt-2">{errorMsg}</Alert>}
    </div>
  )
}