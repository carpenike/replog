import { useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { useConfirm } from '@/lib/useConfirm'
import { Button, buttonVariants } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Alert } from '@/components/ui/alert'

export function EditAthlete() {
  const { id } = useParams<{ id: string }>()
  const athleteId = Number(id)
  const navigate = useNavigate()
  const { confirm, dialog: confirmDialog } = useConfirm()
  const queryClient = useQueryClient()

  const athleteQuery = useQuery({
    queryKey: ['athlete', athleteId],
    queryFn: () => api.getAthlete(athleteId),
    enabled: !isNaN(athleteId),
  })
  const athlete = athleteQuery.data

  const [name, setName] = useState('')
  const [tier, setTier] = useState('')
  const [goal, setGoal] = useState('')
  const [notes, setNotes] = useState('')
  const [dateOfBirth, setDateOfBirth] = useState('')
  const [grade, setGrade] = useState('')
  const [gender, setGender] = useState('')
  const [trackBW, setTrackBW] = useState(true)
  const [initialized, setInitialized] = useState(false)
  const [error, setError] = useState('')

  if (athlete && !initialized) {
    setName(athlete.name)
    setTier(athlete.tier ?? '')
    setGoal(athlete.goal ?? '')
    setNotes(athlete.notes ?? '')
    setDateOfBirth(athlete.date_of_birth ?? '')
    setGrade(athlete.grade ?? '')
    setGender(athlete.gender ?? '')
    setTrackBW(athlete.track_body_weight)
    setInitialized(true)
  }

  const mutation = useMutation({
    mutationFn: () => api.updateAthlete(athleteId, {
      name, tier, goal, notes,
      date_of_birth: dateOfBirth || undefined,
      grade: grade || undefined,
      gender: gender || undefined,
      track_body_weight: trackBW,
    }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athlete', athleteId] })
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
      navigate(`/athletes/${athleteId}`)
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Failed to update athlete')
    },
  })

  const deleteMutation = useMutation({
    mutationFn: () => api.deleteAthlete(athleteId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['athletes'] })
      navigate('/athletes')
    },
  })

  const header = <PageHeader title="Edit Athlete" back={{ to: `/athletes/${athleteId}`, label: athlete?.name ?? 'Athlete' }} />
  if (!athlete) {
    return (
      <div className="max-w-lg">
        {header}
        <QueryState query={athleteQuery} resource="athlete" skeleton="form">{() => null}</QueryState>
      </div>
    )
  }

  return (
    <div className="max-w-lg">
      {header}

      <form onSubmit={(e) => { e.preventDefault(); mutation.mutate() }} className="space-y-4">
        {error && (
          <Alert variant="destructive">{error}</Alert>
        )}

        <div>
          <Label htmlFor="name" >Name <span aria-hidden="true" className="text-destructive">*</span></Label>
          <Input id="name" type="text" value={name} onChange={e => setName(e.target.value)} required />
        </div>

        <div>
          <Label htmlFor="athlete-tier">Tier</Label>
          <Select value={tier} onValueChange={(val) => setTier(val ?? "")}>
            <SelectTrigger id="athlete-tier" className="w-full">
              <SelectValue placeholder="None" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">None</SelectItem>
              <SelectItem value="foundational">Foundational</SelectItem>
              <SelectItem value="intermediate">Intermediate</SelectItem>
              <SelectItem value="sport_performance">Sport Performance</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div>
          <Label htmlFor="goal" >Goal</Label>
          <Textarea id="goal" value={goal} onChange={e => setGoal(e.target.value)} 
            />
        </div>

        <div>
          <Label htmlFor="notes" >Notes</Label>
          <Textarea id="notes" value={notes} onChange={e => setNotes(e.target.value)} 
            />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <Label htmlFor="dob" >Date of Birth</Label>
            <Input id="dob" type="date" value={dateOfBirth} onChange={e => setDateOfBirth(e.target.value)} />
          </div>
          <div>
            <Label htmlFor="athlete-gender">Gender</Label>
            <Select value={gender} onValueChange={(val) => setGender(val ?? "")}>
              <SelectTrigger id="athlete-gender" className="w-full">
                <SelectValue placeholder="Not specified" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">Not specified</SelectItem>
                <SelectItem value="male">Male</SelectItem>
                <SelectItem value="female">Female</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div>
          <Label htmlFor="grade" >Grade</Label>
          <Input id="grade" type="text" value={grade} onChange={e => setGrade(e.target.value)} />
        </div>

        <div className="flex items-center gap-2">
          <Checkbox id="trackBW" checked={trackBW} onCheckedChange={(checked) => setTrackBW(checked)} />
          <Label htmlFor="trackBW">Track body weight</Label>
        </div>

        <div className="flex items-center justify-between pt-2">
          <div className="flex gap-3">
            <Button type="submit" disabled={mutation.isPending}
              >
              {mutation.isPending ? 'Saving...' : 'Save Changes'}
            </Button>
            <Link to={`/athletes/${athleteId}`}
              className={buttonVariants({ variant: "outline" })}>
              Cancel
            </Link>
          </div>
          <Button variant="ghost" type="button" onClick={async () => { if (await confirm({ title: 'Delete Athlete', description: `Delete ${athlete?.name}? This cannot be undone.`, confirmLabel: 'Delete', variant: 'danger' })) deleteMutation.mutate() }}
            className="text-sm text-destructive hover:text-destructive/80">
            Delete
          </Button>
        </div>
      </form>
      {confirmDialog()}
    </div>
  )
}
