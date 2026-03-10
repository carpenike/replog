import { useRef, useState, type ChangeEvent } from 'react'
import { Upload } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface FileUploadProps {
  accept?: string
  disabled?: boolean
  onChange: (file: File) => void
  className?: string
  label?: string
}

export function FileUpload({ accept, disabled, onChange, className, label = 'Choose file' }: FileUploadProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [fileName, setFileName] = useState<string | null>(null)
  const [dragOver, setDragOver] = useState(false)

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (file) {
      setFileName(file.name)
      onChange(file)
    }
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault()
    setDragOver(false)
    const file = e.dataTransfer.files?.[0]
    if (file) {
      setFileName(file.name)
      onChange(file)
    }
  }

  return (
    <div
      className={cn(
        'relative flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-input p-6 transition-colors',
        dragOver && 'border-primary bg-primary/5',
        disabled && 'opacity-50 pointer-events-none',
        className,
      )}
      onDragOver={(e) => { e.preventDefault(); setDragOver(true) }}
      onDragLeave={() => setDragOver(false)}
      onDrop={handleDrop}
    >
      <Upload className="h-8 w-8 text-muted-foreground" />
      {fileName ? (
        <p className="text-sm font-medium">{fileName}</p>
      ) : (
        <p className="text-sm text-muted-foreground">Drag & drop or click to browse</p>
      )}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={() => inputRef.current?.click()}
      >
        {label}
      </Button>
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        onChange={handleChange}
        className="sr-only"
      />
    </div>
  )
}
