import { useId, type ReactNode } from 'react'
import { Label } from '@/components/ui/label'
import { cn } from '@/lib/utils'

/** Props a Field hands to its control so label, hint and error are wired up. */
export interface FieldControlProps {
  id: string
  'aria-describedby'?: string
  'aria-invalid'?: true
  'aria-required'?: true
}

interface FieldProps {
  label: ReactNode
  /**
   * The control's id. Optional: Field generates one, and a render-function
   * child receives it. With a plain child, pass the same id to the control.
   */
  id?: string
  hint?: ReactNode
  error?: ReactNode
  /** Shows a required marker. Also set `required` on the control itself. */
  required?: boolean
  className?: string
  /** Hide the label visually but keep it for assistive tech. */
  srOnlyLabel?: boolean
  children: ReactNode | ((control: FieldControlProps) => ReactNode)
}

/** Build the ids Field uses for its hint and error lines. */
function fieldDescriptionIds(id: string, hasHint: boolean, hasError: boolean): string | undefined {
  const ids = [hasHint ? `${id}-hint` : null, hasError ? `${id}-error` : null].filter(Boolean)
  return ids.length ? ids.join(' ') : undefined
}

/**
 * Label + control + hint + error, with the label associated to the control
 * (htmlFor/id) and the hint and error linked via aria-describedby. Use it for
 * every form control, including Select triggers (pass the id to SelectTrigger).
 */
export function Field({ label, id, hint, error, required, className, srOnlyLabel, children }: FieldProps) {
  const autoId = useId()
  const controlId = id ?? `field-${autoId.replace(/:/g, '')}`
  const describedBy = fieldDescriptionIds(controlId, !!hint, !!error)
  const control: FieldControlProps = {
    id: controlId,
    ...(describedBy ? { 'aria-describedby': describedBy } : {}),
    ...(error ? { 'aria-invalid': true as const } : {}),
    ...(required ? { 'aria-required': true as const } : {}),
  }
  return (
    <div className={cn('space-y-1.5', className)}>
      <Label htmlFor={controlId} className={srOnlyLabel ? 'sr-only' : undefined}>
        {label}
        {required && (
          <span aria-hidden="true" className="text-destructive">*</span>
        )}
      </Label>
      {typeof children === 'function' ? children(control) : children}
      {hint && <p id={`${controlId}-hint`} className="text-xs text-muted-foreground">{hint}</p>}
      {error && <p id={`${controlId}-error`} role="alert" className="text-xs text-destructive">{error}</p>}
    </div>
  )
}
