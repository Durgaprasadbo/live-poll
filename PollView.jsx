import { useEffect, useRef, useState, useCallback } from 'react'
import { useParams } from 'react-router-dom'
import client, { WS_BASE, getVoterId } from '../api/client.js'
import { useAuth } from '../context/AuthContext.jsx'

export default function PollView() {
  const { id } = useParams()
  const { user } = useAuth()

  const [poll, setPoll] = useState(null)
  const [counts, setCounts] = useState({})
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [votedOption, setVotedOption] = useState(() => localStorage.getItem(`voted:${id}`))
  const [wsConnected, setWsConnected] = useState(false)
  const wsRef = useRef(null)

  const applyResult = useCallback((result) => {
    setCounts(result.counts || {})
    setTotal(result.total || 0)
  }, [])

  // Load poll definition + initial results once.
  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [pollRes, resultsRes] = await Promise.all([
          client.get(`/polls/${id}`),
          client.get(`/polls/${id}/results`),
        ])
        if (cancelled) return
        setPoll(pollRes.data)
        applyResult(resultsRes.data)
      } catch (err) {
        if (!cancelled) setError(err.response?.data?.error || 'Poll not found')
      }
    }
    load()
    return () => { cancelled = true }
  }, [id, applyResult])

  // Open a websocket for live updates; auto-reconnect if it drops.
  useEffect(() => {
    let cancelled = false
    let socket
    let retryTimer

    function connect() {
      socket = new WebSocket(`${WS_BASE}/ws/polls/${id}`)
      wsRef.current = socket

      socket.onopen = () => !cancelled && setWsConnected(true)
      socket.onclose = () => {
        if (cancelled) return
        setWsConnected(false)
        retryTimer = setTimeout(connect, 2000)
      }
      socket.onerror = () => socket.close()
      socket.onmessage = (event) => {
        try {
          const result = JSON.parse(event.data)
          applyResult(result)
        } catch { /* ignore malformed frame */ }
      }
    }

    connect()
    return () => {
      cancelled = true
      clearTimeout(retryTimer)
      socket?.close()
    }
  }, [id, applyResult])

  async function handleVote(optionId) {
    setError('')
    try {
      await client.post(`/polls/${id}/vote`, { option_id: optionId }, {
        headers: { 'X-Voter-Id': getVoterId() },
      })
      localStorage.setItem(`voted:${id}`, optionId)
      setVotedOption(optionId)
    } catch (err) {
      setError(err.response?.data?.error || 'Could not submit vote')
    }
  }

  async function handleClose() {
    try {
      await client.post(`/polls/${id}/close`)
      setPoll(p => ({ ...p, is_active: false }))
    } catch {
      setError('Could not close poll')
    }
  }

  if (error && !poll) {
    return <div className="container"><div className="error-box">{error}</div></div>
  }
  if (!poll) {
    return <div className="container"><p className="subtitle">Loading…</p></div>
  }

  const isOwner = user && poll.owner_id === user.id
  const shareUrl = `${window.location.origin}/polls/${id}`

  return (
    <div className="container" style={{ maxWidth: 560 }}>
      <div className="status-pill">
        <span className="live-dot" style={{ background: wsConnected ? 'var(--success)' : 'var(--danger)' }} />
        {wsConnected ? 'Live' : 'Reconnecting…'}
        <span className="badge" style={{ marginLeft: 10 }}>{poll.is_active ? 'Active' : 'Closed'}</span>
      </div>

      <h1>{poll.question}</h1>
      <p className="subtitle">{total} vote{total === 1 ? '' : 's'} so far</p>

      <div className="share-box">
        <input readOnly value={shareUrl} onFocus={e => e.target.select()} />
        <button className="btn btn-secondary" onClick={() => navigator.clipboard.writeText(shareUrl)}>Copy</button>
      </div>

      {error && <div className="error-box">{error}</div>}

      <div className="card">
        {poll.options.map(opt => {
          const count = counts[opt.id] || 0
          const pct = total > 0 ? Math.round((count / total) * 100) : 0
          const showResults = votedOption || !poll.is_active

          if (!showResults) {
            return (
              <button
                key={opt.id}
                className="option-btn"
                onClick={() => handleVote(opt.id)}
                disabled={!poll.is_active}
              >
                {opt.text}
              </button>
            )
          }

          return (
            <div className="bar-row" key={opt.id}>
              <div className="bar-label">
                <span>{opt.text}{votedOption === opt.id ? ' ✓' : ''}</span>
                <span>{pct}% ({count})</span>
              </div>
              <div className="bar-track">
                <div
                  className={`bar-fill ${votedOption === opt.id ? 'voted' : ''}`}
                  style={{ width: `${pct}%` }}
                />
              </div>
            </div>
          )
        })}

        {!poll.is_active && <p className="subtitle" style={{ marginTop: 12 }}>This poll is closed to new votes.</p>}
        {poll.is_active && votedOption && <p className="subtitle" style={{ marginTop: 12 }}>Thanks for voting — results update live.</p>}
      </div>

      {isOwner && poll.is_active && (
        <button className="btn btn-danger" onClick={handleClose}>Close poll</button>
      )}
    </div>
  )
}
