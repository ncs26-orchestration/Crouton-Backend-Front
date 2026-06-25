import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

export const Route = createFileRoute('/')({ component: Home })

type Note = {
  id: number
  title: string
  body: string
  created_at: string
}

// The browser hits these paths relative to the same origin. nginx routes
// /api/* to the Go backend, everything else to this SSR app.
async function fetchNotes(): Promise<Note[]> {
  const res = await fetch('/api/notes')
  if (!res.ok) throw new Error(`GET /api/notes failed: ${res.status}`)
  return res.json()
}

async function createNote(title: string, body: string): Promise<Note> {
  const res = await fetch('/api/notes', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, body }),
  })
  if (!res.ok) throw new Error(`POST /api/notes failed: ${res.status}`)
  return res.json()
}

function Home() {
  const [notes, setNotes] = useState<Note[]>([])
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const load = () => {
    fetchNotes()
      .then(setNotes)
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) return
    try {
      await createNote(title.trim(), body.trim())
      setTitle('')
      setBody('')
      setError(null)
      load()
    } catch (err) {
      setError(String(err))
    }
  }

  return (
    <div className="mx-auto max-w-2xl p-8">
      <h1 className="text-4xl font-bold">Notes</h1>
      <p className="mt-2 text-gray-500">
        React 19 + TanStack Start, talking to Go/Echo + Postgres through nginx.
      </p>

      <form onSubmit={onSubmit} className="mt-6 flex flex-col gap-2">
        <input
          className="rounded border px-3 py-2"
          placeholder="Title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
        <textarea
          className="rounded border px-3 py-2"
          placeholder="Body"
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
        <button
          type="submit"
          className="self-start rounded bg-black px-4 py-2 text-white"
        >
          Add note
        </button>
      </form>

      {error && <p className="mt-4 text-red-600">{error}</p>}
      {loading && <p className="mt-4">Loading...</p>}

      <ul className="mt-6 flex flex-col gap-3">
        {notes.map((n) => (
          <li key={n.id} className="rounded border p-4">
            <div className="font-semibold">{n.title}</div>
            {n.body && <div className="mt-1 text-gray-600">{n.body}</div>}
            <div className="mt-2 text-xs text-gray-400">{n.created_at}</div>
          </li>
        ))}
      </ul>
    </div>
  )
}
