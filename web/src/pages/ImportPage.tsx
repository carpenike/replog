import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { api } from '@/api/client'
import { Spinner } from '@/components/ui'

interface MappingItem {
  name: string
  mapped_id: number
  create: boolean
}

interface UploadResult {
  format: string
  exercises: MappingItem[]
  equipment?: MappingItem[]
}

interface ImportResult {
  workouts_created: number
  sets_created: number
  exercises_created: number
}

export function ImportPage() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)

  const [step, setStep] = useState<'upload' | 'map' | 'result'>('upload')
  const [format, setFormat] = useState('strong')
  const [uploadResult, setUploadResult] = useState<UploadResult | null>(null)
  const [mappings, setMappings] = useState<MappingItem[]>([])
  const [importResult, setImportResult] = useState<ImportResult | null>(null)

  const { data: athlete } = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })

  const { data: exercises } = useQuery({
    queryKey: ['exercises'],
    queryFn: () => api.listExercises(),
    enabled: step === 'map',
  })

  const uploadMutation = useMutation({
    mutationFn: async (file: File) => {
      const formData = new FormData()
      formData.append('file', file)
      formData.append('format', format)
      const res = await fetch(`/api/athletes/${athleteId}/import/upload`, {
        method: 'POST',
        body: formData,
        credentials: 'include',
        headers: { 'Accept': 'application/json' },
      })
      if (!res.ok) throw new Error((await res.json()).error ?? 'Upload failed')
      return res.json() as Promise<UploadResult>
    },
    onSuccess: (data) => {
      setUploadResult(data)
      setMappings(data.exercises)
      setStep('map')
    },
  })

  const executeMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/athletes/${athleteId}/import/execute`, {
        method: 'POST',
        body: JSON.stringify({ exercises: mappings }),
        credentials: 'include',
        headers: { 'Accept': 'application/json', 'Content-Type': 'application/json' },
      })
      if (!res.ok) throw new Error((await res.json()).error ?? 'Import failed')
      return res.json() as Promise<ImportResult>
    },
    onSuccess: (data) => {
      setImportResult(data)
      setStep('result')
    },
  })

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (file) uploadMutation.mutate(file)
  }

  function updateMapping(index: number, mappedId: number) {
    setMappings(prev => prev.map((m, i) =>
      i === index ? { ...m, mapped_id: mappedId, create: mappedId === 0 } : m
    ))
  }

  return (
    <div className="max-w-2xl">
      <p className="text-sm text-muted-foreground mb-1">
        <Link to={`/athletes/${athleteId}`} className="hover:text-foreground">{athlete?.name ?? 'Athlete'}</Link>
        {' / Import'}
      </p>
      <h1 className="text-2xl font-bold mb-6">Import Workouts</h1>

      {/* Step 1: Upload */}
      {step === 'upload' && (
        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium mb-1">Format</label>
            <select value={format} onChange={e => setFormat(e.target.value)}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm">
              <option value="strong">Strong CSV</option>
              <option value="hevy">Hevy CSV</option>
              <option value="replog">RepLog JSON</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium mb-1">File</label>
            <input type="file" accept=".csv,.json" onChange={handleFileChange}
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm" />
          </div>
          {uploadMutation.isPending && <Spinner />}
          {uploadMutation.isError && (
            <p className="text-sm text-destructive">{(uploadMutation.error as Error).message}</p>
          )}
        </div>
      )}

      {/* Step 2: Map exercises */}
      {step === 'map' && uploadResult && (
        <div>
          <p className="text-sm text-muted-foreground mb-4">
            Found {mappings.length} exercise{mappings.length !== 1 ? 's' : ''} in your {uploadResult.format} file.
            Map them to existing exercises or create new ones.
          </p>

          <div className="space-y-3 mb-6">
            {mappings.map((m, i) => (
              <div key={i} className="flex items-center gap-3 rounded-lg border border-border bg-card p-3">
                <p className="text-sm font-medium w-48 truncate" title={m.name}>{m.name}</p>
                <span className="text-muted-foreground">→</span>
                <select
                  value={m.mapped_id}
                  onChange={e => updateMapping(i, parseInt(e.target.value))}
                  className="flex-1 rounded-md border border-border bg-background px-3 py-1.5 text-sm"
                >
                  <option value={0}>Create new</option>
                  {exercises?.map(ex => (
                    <option key={ex.id} value={ex.id}>{ex.name}</option>
                  ))}
                </select>
              </div>
            ))}
          </div>

          <div className="flex gap-3">
            <button onClick={() => executeMutation.mutate()}
              disabled={executeMutation.isPending}
              className="rounded-md bg-primary px-6 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50">
              {executeMutation.isPending ? 'Importing...' : 'Import'}
            </button>
            <button onClick={() => setStep('upload')}
              className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
              Back
            </button>
          </div>
          {executeMutation.isError && (
            <p className="text-sm text-destructive mt-2">{(executeMutation.error as Error).message}</p>
          )}
        </div>
      )}

      {/* Step 3: Result */}
      {step === 'result' && importResult && (
        <div className="rounded-lg border border-border bg-card p-6 text-center">
          <span className="text-4xl block mb-3">✅</span>
          <h2 className="text-lg font-semibold mb-2">Import Complete!</h2>
          <div className="text-sm text-muted-foreground space-y-1">
            <p>{importResult.workouts_created} workout{importResult.workouts_created !== 1 ? 's' : ''} imported</p>
            <p>{importResult.sets_created} set{importResult.sets_created !== 1 ? 's' : ''} created</p>
            {importResult.exercises_created > 0 && (
              <p>{importResult.exercises_created} new exercise{importResult.exercises_created !== 1 ? 's' : ''} created</p>
            )}
          </div>
          <div className="mt-4 flex gap-3 justify-center">
            <Link to={`/athletes/${athleteId}/workouts`}
              className="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90">
              View Workouts
            </Link>
            <button onClick={() => { setStep('upload'); setImportResult(null); setUploadResult(null) }}
              className="rounded-md border border-border px-4 py-2 text-sm hover:bg-accent transition-colors">
              Import More
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
