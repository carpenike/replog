import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/client'
import { Field } from '@/components/Field'
import { PageHeader } from '@/components/PageHeader'
import { QueryState } from '@/components/QueryState'
import { CardList, CardListItem } from '@/components/CardList'
import { Input } from '@/components/ui/input'

describe('PageHeader', () => {
  it('renders the h1, a back link and sets the document title', () => {
    render(
      <MemoryRouter>
        <PageHeader title="Body Weight" back={{ to: '/athletes/7', label: 'Sam' }} actions={<button type="button">Log</button>} />
      </MemoryRouter>,
    )
    expect(screen.getByRole('heading', { level: 1, name: 'Body Weight' })).toBeTruthy()
    expect(screen.getByRole('link', { name: 'Sam' }).getAttribute('href')).toBe('/athletes/7')
    expect(screen.getByRole('button', { name: 'Log' })).toBeTruthy()
    expect(document.title).toBe('Body Weight · RepLog')
  })

  it('prefers pageTitle and keeps the base title while it is null', () => {
    const { rerender } = render(<MemoryRouter><PageHeader title="Edit" pageTitle={null} /></MemoryRouter>)
    expect(document.title).toBe('RepLog')
    rerender(<MemoryRouter><PageHeader title="Edit" pageTitle="Edit Sam" /></MemoryRouter>)
    expect(document.title).toBe('Edit Sam · RepLog')
  })
})

describe('QueryState', () => {
  const renderState = (query: { data: string[] | undefined; error: unknown; refetch?: () => unknown }) =>
    render(
      <MemoryRouter>
        <QueryState
          query={query}
          resource="athletes"
          empty={{ title: 'No athletes yet', action: 'New Athlete', actionTo: '/athletes/new' }}
        >
          {data => <ul>{data.map(d => <li key={d}>{d}</li>)}</ul>}
        </QueryState>
      </MemoryRouter>,
    )

  it('shows a skeleton while loading', () => {
    renderState({ data: undefined, error: null })
    expect(screen.getByRole('status', { name: 'Loading' })).toBeTruthy()
  })

  it('shows the error with a working Retry', async () => {
    const refetch = vi.fn()
    renderState({ data: undefined, error: new Error('Boom'), refetch })
    expect(screen.getByText('Boom')).toBeTruthy()
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(refetch).toHaveBeenCalledOnce()
  })

  it('treats a 404 as not found, not as an error', () => {
    renderState({ data: undefined, error: new ApiError('nope', 404) })
    expect(screen.getByText('No athletes found')).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
  })

  it('shows the empty state with its call to action', () => {
    renderState({ data: [], error: null })
    expect(screen.getByText('No athletes yet')).toBeTruthy()
    expect(screen.getByRole('link', { name: 'New Athlete' }).getAttribute('href')).toBe('/athletes/new')
  })

  it('renders the data', () => {
    renderState({ data: ['Sam', 'Ava'], error: null })
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
  })
})

describe('Field', () => {
  it('associates the label, hint and error with the control', () => {
    render(
      <Field label="Weight" required hint="In pounds" error="Required">
        {control => <Input type="number" {...control} />}
      </Field>,
    )
    const input = screen.getByLabelText(/Weight/)
    expect(input.getAttribute('aria-required')).toBe('true')
    expect(input.getAttribute('aria-invalid')).toBe('true')
    const described = input.getAttribute('aria-describedby')!.split(' ')
    expect(described.map(id => document.getElementById(id)?.textContent)).toEqual(['In pounds', 'Required'])
  })

  it('works with a plain child that carries the same id', () => {
    render(<Field label="Notes" id="notes"><Input id="notes" /></Field>)
    expect(screen.getByLabelText('Notes').id).toBe('notes')
  })
})

describe('CardList', () => {
  it('makes the whole card a real link', () => {
    render(
      <MemoryRouter>
        <CardList label="Athletes">
          <CardListItem to="/athletes/7" title="Sam" subtitle="Last workout Today" />
        </CardList>
      </MemoryRouter>,
    )
    const link = screen.getByRole('link', { name: /Sam/ })
    expect(link.getAttribute('href')).toBe('/athletes/7')
  })
})
