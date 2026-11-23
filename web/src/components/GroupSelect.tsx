import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { GROUP_KEY_OPTIONS } from '@/lib/groups'

const NO_GROUP = 'none'

/** Group picker for an exercise or set. "none" means not grouped. */
export function GroupSelect({ id, value, onChange, disabled, className }: {
  id: string
  value: string | null
  onChange: (key: string | null) => void
  disabled?: boolean
  className?: string
}) {
  return (
    <Select value={value ?? NO_GROUP} onValueChange={(v) => onChange(v == null || v === NO_GROUP ? null : v)} disabled={disabled}>
      <SelectTrigger id={id} className={className ?? 'w-full'}>
        <SelectValue>
          {(v: string | null) => (!v || v === NO_GROUP ? 'No group' : `Group ${v}`)}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={NO_GROUP}>No group</SelectItem>
        {GROUP_KEY_OPTIONS.map(k => <SelectItem key={k} value={k}>Group {k}</SelectItem>)}
      </SelectContent>
    </Select>
  )
}

