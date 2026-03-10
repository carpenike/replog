import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { SettingCategoryData } from '@/api/types'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
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
    <div className="max-w-2xl">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>
      <div className="space-y-8">
        {categories?.map((cat: SettingCategoryData) => (
          <div key={cat.category}>
            <h2 className="text-lg font-semibold mb-3 text-foreground">{cat.category}</h2>
            <div className="space-y-3">
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
            </div>
          </div>
        ))}
      </div>
      {/* Test Connections */}
      <Card className="mt-8">
        <CardContent>
        <h2 className="text-lg font-semibold mb-3">Test Connections</h2>
        <div className="flex flex-wrap gap-3">
          <TestButton label="Test LLM" onClick={() => api.testLLMConnection()} />
          <TestButton label="Test Notifications" onClick={() => api.testNotifyConnection()} />
        </div>
        </CardContent>
      </Card>
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
  function handleSave() {
    onSave(draft)
    setEditing(false)
  }
  return (
    <Card size="sm">
      <CardContent>
      <div className="flex items-center justify-between">
        <div className="flex-1">
          <p className="text-sm font-medium capitalize">{label}</p>
          <p className="text-xs text-muted-foreground font-mono">{settingKey}</p>
        </div>
        <div className="flex items-center gap-2">
          {source !== 'default' && (
            <span className={`text-xs px-1.5 py-0.5 rounded ${
              source === 'env' ? 'bg-primary/10 text-primary' : 'bg-muted text-muted-foreground'
            }`}>
              {source}
            </span>
          )}
          {isSaved && (
            <span className="text-xs text-success">Saved ✓</span>
          )}
        </div>
      </div>
      {editing ? (
        <div className="mt-2 flex gap-2">
          <Input type="text" value={draft} onChange={e => setDraft(e.target.value)} className="font-mono" />
          <Button onClick={handleSave}
            >
            Save
          </Button>
          <Button variant="ghost" onClick={() => { setEditing(false); setDraft(value) }}
            >
            Cancel
          </Button>
        </div>
      ) : (
        <div className="mt-1 flex items-center gap-2">
          <p className="text-sm font-mono text-muted-foreground">{masked || value || '(empty)'}</p>
          {!readOnly && (
            <Button variant="ghost" onClick={() => setEditing(true)}
              >
              Edit
            </Button>
          )}
        </div>
      )}
      </CardContent>
    </Card>
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
      <Button variant="ghost" onClick={handleTest} disabled={status === 'testing'} className={`rounded-md px-4 py-2 text-sm font-medium transition-colors ${ status === 'success' ? 'bg-success/20 text-success' : status === 'error' ? 'bg-destructive/20 text-destructive' : 'border border-border hover:bg-accent' } disabled:opacity-50`}>
        {status === 'testing' ? 'Testing...' :
         status === 'success' ? '✓ Connected' :
         status === 'error' ? '✗ Failed' :
         label}
      </Button>
      {errorMsg && <p className="text-xs text-destructive mt-1">{errorMsg}</p>}
    </div>
  )
}