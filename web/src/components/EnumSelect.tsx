import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export interface EnumOption {
  value: string
  label: string
}

interface EnumSelectProps {
  value: string
  onChange: (value: string) => void
  options: EnumOption[]
  placeholder?: string
  required?: boolean
  className?: string
}

/**
 * EnumSelect is a thin wrapper over the shadcn Select for fixed option lists,
 * used by the multi-modal logbook forms (HOF-011). It renders the selected
 * option's label via the SelectValue render-child the base Select expects.
 */
export function EnumSelect({ value, onChange, options, placeholder = 'Select...', required, className }: EnumSelectProps) {
  return (
    <Select value={value || null} onValueChange={(val) => onChange(val ?? '')} required={required}>
      <SelectTrigger className={className ?? 'w-full'}>
        <SelectValue placeholder={placeholder}>
          {(val: string | null) => {
            if (!val) return placeholder
            return options.find(o => o.value === val)?.label ?? val
          }}
        </SelectValue>
      </SelectTrigger>
      <SelectContent>
        {options.map(o => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
      </SelectContent>
    </Select>
  )
}
