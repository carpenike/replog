import { useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation } from '@tanstack/react-query'
import { api } from '@/api/client'
import { ArrowRight, Loader2 } from 'lucide-react'
import { PageHeader } from '@/components/PageHeader'
import { Label } from '@/components/ui/label'
import { FileUpload } from '@/components/FileUpload'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Card, CardContent } from '@/components/ui/card'
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
  const navigate = useNavigate()
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
  function updateMapping(index: number, mappedId: number) {
    setMappings(prev => prev.map((m, i) =>
      i === index ? { ...m, mapped_id: mappedId, create: mappedId === 0 } : m
    ))
  }
  return (
    <div className="max-w-2xl">
      <PageHeader title="Import Workouts" back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }} />
      {/* Step 1: Upload */}
      {step === 'upload' && (
        <div className="space-y-4">
          <div>
            <Label htmlFor="import-format">Format</Label>
            <Select value={format} onValueChange={(val) => setFormat(val ?? "strong")}>
              <SelectTrigger id="import-format" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="strong">Strong CSV</SelectItem>
                <SelectItem value="hevy">Hevy CSV</SelectItem>
                <SelectItem value="replog">RepLog JSON</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <Label htmlFor="import-file">File</Label>
            <FileUpload id="import-file" accept=".csv,.json" onChange={(file) => uploadMutation.mutate(file)}
               label="Select file" />
          </div>
          {uploadMutation.isPending && (
            <p role="status" className="flex items-center gap-2 text-sm text-muted-foreground">
              <Loader2 className="size-4 animate-spin" aria-hidden="true" /> Reading file…
            </p>
          )}
          {uploadMutation.isError && (
            <p role="alert" className="text-sm text-destructive">{(uploadMutation.error as Error).message}</p>
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
              <Card key={`${m.name}-${i}`} size="sm">
                <CardContent className="flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3">
                <p id={`import-map-${i}`} className="text-sm font-medium sm:w-48 truncate" title={m.name}>{m.name}</p>
                <ArrowRight className="hidden size-4 text-muted-foreground sm:block" aria-hidden="true" />
                <Select
                  value={String(m.mapped_id)}
                  onValueChange={(val) => updateMapping(i, parseInt(val ?? '0'))}
                >
                  <SelectTrigger aria-labelledby={`import-map-${i}`} className="w-full sm:flex-1">
                    <SelectValue>
                      {(value: string | null) => {
                        if (!value || value === '0') return 'Create new'
                        return exercises?.find(ex => String(ex.id) === value)?.name ?? value
                      }}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="0">Create new</SelectItem>
                    {exercises?.map(ex => (
                      <SelectItem key={ex.id} value={String(ex.id)}>{ex.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                </CardContent>
              </Card>
            ))}
          </div>
          <div className="flex gap-3">
            <Button onClick={() => executeMutation.mutate()}
              disabled={executeMutation.isPending}
              >
              {executeMutation.isPending ? 'Importing...' : 'Import'}
            </Button>
            <Button variant="ghost" onClick={() => setStep('upload')}
              >
              Back
            </Button>
          </div>
          {executeMutation.isError && (
            <p role="alert" className="text-sm text-destructive mt-2">{(executeMutation.error as Error).message}</p>
          )}
        </div>
      )}
      {/* Step 3: Result */}
      {step === 'result' && importResult && (
        <Card className="text-center">
          <CardContent>
          <span className="text-4xl block mb-3" aria-hidden="true">✅</span>
          <h2 className="text-lg font-semibold mb-2">Import Complete!</h2>
          <div className="text-sm text-muted-foreground space-y-1">
            <p>{importResult.workouts_created} workout{importResult.workouts_created !== 1 ? 's' : ''} imported</p>
            <p>{importResult.sets_created} set{importResult.sets_created !== 1 ? 's' : ''} created</p>
            {importResult.exercises_created > 0 && (
              <p>{importResult.exercises_created} new exercise{importResult.exercises_created !== 1 ? 's' : ''} created</p>
            )}
          </div>
          <div className="mt-4 flex gap-3 justify-center">
            <Button onClick={() => navigate(`/athletes/${athleteId}/workouts`)}>
              View Workouts
            </Button>
            <Button variant="ghost" onClick={() => { setStep('upload'); setImportResult(null); setUploadResult(null) }}
              >
              Import More
            </Button>
          </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}