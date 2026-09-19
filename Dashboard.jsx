import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import client from '../api/client.js'

export default function Dashboard() {
  const [polls, setPolls] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    client.get('/polls')
      .then(res => setPolls(res.data))
      .catch(() => setError('Could not load your polls'))
      .finally(() => setLoading(false))
  }, [])

  return (
    <div className="container">
      <h1>Your polls</h1>
      <p className="subtitle">Everything you've created.</p>
      {error && <div className="error-box">{error}</div>}
      <div className="card">
        {loading && <p className="subtitle">Loading…</p>}
        {!loading && polls.length === 0 && (
          <div className="empty-state">
            <p>No polls yet.</p>
            <Link to="/create" className="btn">Create your first poll</Link>
          </div>
        )}
        {polls.map(p => (
          <div className="poll-list-item" key={p.id}>
            <div>
              <div>{p.question}</div>
              <span className={`badge ${p.is_active ? 'active' : 'closed'}`}>
                {p.is_active ? 'Active' : 'Closed'}
              </span>
            </div>
            <Link to={`/polls/${p.id}`} className="btn btn-secondary">View</Link>
          </div>
        ))}
      </div>
    </div>
  )
}
