import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { ApiError } from '@/api/client'
import { Spinner } from '@/components/ui'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Card, CardContent } from '@/components/ui/card'
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
          <Card>
            <CardContent>
            <h2 className="font-semibold mb-2">Export Catalog</h2>
            <p className="text-sm text-muted-foreground mb-4">
              Download the full exercise, equipment, and program catalog as JSON.
            </p>
            <Button onClick={downloadCatalog}
              >
              Download JSON
            </Button>
            </CardContent>
          </Card>
          <Card>
            <CardContent>
            <h2 className="font-semibold mb-2">Import Catalog</h2>
            <p className="text-sm text-muted-foreground mb-4">
              Upload a catalog JSON file to add exercises, equipment, and programs.
            </p>
            <Input type="file" accept=".json" onChange={handleFileChange}
               />
            {uploadMutation.isPending && <Spinner className="mt-2" />}
            </CardContent>
          </Card>
        </div>
      )}
      {step === 'importing' && uploadMutation.data && (
        <div>
          <Card className="mb-6">
            <CardContent>
            <h2 className="font-semibold mb-2">Catalog Preview</h2>
            <div className="grid grid-cols-3 gap-4 text-sm">
              <div><p className="text-muted-foreground">Exercises</p><p className="text-lg font-bold">{uploadMutation.data.exercises}</p></div>
              <div><p className="text-muted-foreground">Equipment</p><p className="text-lg font-bold">{uploadMutation.data.equipment}</p></div>
              <div><p className="text-muted-foreground">Programs</p><p className="text-lg font-bold">{uploadMutation.data.programs}</p></div>
            </div>
            </CardContent>
          </Card>
          <div className="flex gap-3">
            <Button variant="ghost" onClick={() => executeMutation.mutate()}
              disabled={executeMutation.isPending}
              >
              {executeMutation.isPending ? 'Importing...' : 'Import Catalog'}
            </Button>
            <Button variant="ghost" onClick={() => { setStep('menu'); uploadMutation.reset() }}
              >
              Cancel
            </Button>
          </div>
        </div>
      )}
      {step === 'result' && importResult && (
        <Card className="text-center">
          <CardContent>
          <span className="text-4xl block mb-3">✅</span>
          <h2 className="text-lg font-semibold mb-2">Catalog Imported!</h2>
          <div className="text-sm text-muted-foreground space-y-1">
            {importResult.exercises_created > 0 && <p>{importResult.exercises_created} exercises</p>}
            {importResult.equipment_created > 0 && <p>{importResult.equipment_created} equipment</p>}
            {importResult.programs_created > 0 && <p>{importResult.programs_created} programs</p>}
            {importResult.prescribed_sets > 0 && <p>{importResult.prescribed_sets} prescribed sets</p>}
            {importResult.progression_rules > 0 && <p>{importResult.progression_rules} progression rules</p>}
          </div>
          <Button variant="ghost" onClick={() => { setStep('menu'); setImportResult(null) }}
            >
            Done
          </Button>
          </CardContent>
        </Card>
      )}
    </div>
  )
}