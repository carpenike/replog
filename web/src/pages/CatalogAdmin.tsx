import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'

interface ImportResult {
  exercises_created: number
  equipment_created: number
  programs_created: number
  prescribed_sets: number
  progression_rules: number
}

export function CatalogAdmin() {
  const [step, setStep] = useState<'menu' | 'importing' | 'result'>('menu')
  const [importResult, setImportResult] = useState<ImportResult | null>(null)
  const [error, setError] = useState('')

  const uploadMutation = useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      const res = await fetch('/api/catalog/import/upload', {
        method: 'POST',
        body: formData,
        credentials: 'include',
        headers: { 'Accept': 'application/json' },
      })
      if (!res.ok) throw new ApiError((await res.json()).error ?? 'Upload failed', res.status)
      return res.json() as Promise<{ exercises: number; equipment: number; programs: number }>
    },
  })

  const executeMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch('/api/catalog/import/execute', {
        method: 'POST',
        credentials: 'include',
        headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
        body: '{}',
      })
      if (!res.ok) throw new ApiError((await res.json()).error ?? 'Import failed', res.status)
      return res.json() as Promise<ImportResult>
    },
    onSuccess: (data) => {
      setImportResult(data)
      setStep('result')
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Import failed')
    },
  })

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (file) {
      setError('')
      uploadMutation.mutate(file, {
        onSuccess: () => setStep('importing'),
        onError: (err) => setError(err instanceof ApiError ? err.message : 'Upload failed'),
      })
    }
  }

  function downloadCatalog() {
    const a = document.createElement('a')
    a.href = '/api/catalog/export'
    a.download = 'replog-catalog.json'
    a.click()
  }

  return (
    <div className="max-w-2xl">
      <h1 className="text-2xl font-bold mb-6">Catalog Administration</h1>

      {error && (
        <div className="rounded-md bg-destructive/10 border border-destructive/30 p-3 text-sm text-destructive mb-4">
          {error}
        </div>
      )}

      {step === 'menu' && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="rounded-lg border border-border bg-card p-6">
            <h2 className="font-semibold mb-2">Export Catalog</h2>
            <p className="text-sm text-muted-foreground mb-4">
              Download the full exercise, equipment, and program catalog as JSON.
            </p>
            <button onClick={downloadCatalog}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
              Download JSON
            </button>
          </div>

          <div className="rounded-lg border border-border bg-card p-6">
            <h2 className="font-semibold mb-2">Import Catalog</h2>
            <p className="text-sm text-muted-foreground mb-4">
              Upload a catalog JSON file to add exercises, equipment, and programs.
            </p>
            <input type="file" accept=".json" onChange={handleFileChange}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
            {uploadMutation.isPending && <Spinner className="mt-2" />}
          </div>
        </div>
      )}

      {step === 'importing' && uploadMutation.data && (
        <div>
          <div className="rounded-lg border border-border bg-card p-4 mb-6">
            <h2 className="font-semibold mb-2">Catalog Preview</h2>
            <div className="grid grid-cols-3 gap-4 text-sm">
              <div><p className="text-muted-foreground">Exercises</p><p className="text-lg font-bold">{uploadMutation.data.exercises}</p></div>
              <div><p className="text-muted-foreground">Equipment</p><p className="text-lg font-bold">{uploadMutation.data.equipment}</p></div>
              <div><p className="text-muted-foreground">Programs</p><p className="text-lg font-bold">{uploadMutation.data.programs}</p></div>
            </div>
          </div>

          <div className="flex gap-3">
            <button onClick={() => executeMutation.mutate()}
              disabled={executeMutation.isPending}
              className="rounded-md bg-primary px-6 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              {executeMutation.isPending ? 'Importing...' : 'Import Catalog'}
            </button>
            <button onClick={() => { setStep('menu'); uploadMutation.reset() }}
              className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
              Cancel
            </button>
          </div>
        </div>
      )}

      {step === 'result' && importResult && (
        <div className="rounded-lg border border-border bg-card p-6 text-center">
          <span className="text-4xl block mb-3">✅</span>
          <h2 className="text-lg font-semibold mb-2">Catalog Imported!</h2>
          <div className="text-sm text-muted-foreground space-y-1">
            {importResult.exercises_created > 0 && <p>{importResult.exercises_created} exercises</p>}
            {importResult.equipment_created > 0 && <p>{importResult.equipment_created} equipment</p>}
            {importResult.programs_created > 0 && <p>{importResult.programs_created} programs</p>}
            {importResult.prescribed_sets > 0 && <p>{importResult.prescribed_sets} prescribed sets</p>}
            {importResult.progression_rules > 0 && <p>{importResult.progression_rules} progression rules</p>}
          </div>
          <button onClick={() => { setStep('menu'); setImportResult(null) }}
            className="mt-4 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
            Done
          </button>
        </div>
      )}
    </div>
  )
}
