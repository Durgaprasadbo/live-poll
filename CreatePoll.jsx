import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import client from '../api/client.js'

export default function CreatePoll() {
  const [question, setQuestion] = useState('')
  const [options, setOptions] = useState(['', ''])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const navigate = useNavigate()

  function updateOption(i, value) {
    const next = [...options]
    next[i] = value
    setOptions(next)
  }

  function addOption() {
    if (options.length >= 10) return
    setOptions([...options, ''])
  }

  function removeOption(i) {
    if (options.length <= 2) return
    setOptions(options.filter((_, idx) => idx !== i))
  }

  async function handleSubmit(e) {
    e.preventDefault()
    setError('')
    const cleaned = options.map(o => o.trim()).filter(Boolean)
    if (cleaned.length < 2) {
      setError('Add at least 2 non-empty options')
      return
    }
    setLoading(true)
    try {
      const res = await client.post('/polls', { question: question.trim(), options: cleaned })
      navigate(`/polls/${res.data.id}`)
    } catch (err) {
      setError(err.response?.data?.error || 'Could not create poll')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="container" style={{ maxWidth: 520 }}>
      <h1>Create a poll</h1>
      <p className="subtitle">Ask a question, add options, share the link.</p>
      {error && <div className="error-box">{error}</div>}
      <form onSubmit={handleSubmit} className="card">
        <label>Question</label>
        <input
          value={question}
          onChange={e => setQuestion(e.target.value)}
          placeholder="What should we build next?"
          maxLength={300}
          required
        />

        <label>Options</label>
        {options.map((opt, i) => (
          <div className="option-row" key={i}>
            <input
              value={opt}
              onChange={e => updateOption(i, e.target.value)}
              placeholder={`Option ${i + 1}`}
              maxLength={150}
            />
            {options.length > 2 && (
              <button type="button" className="remove-btn" onClick={() => removeOption(i)}>×</button>
            )}
          </div>
        ))}

        {options.length < 10 && (
          <button type="button" className="btn btn-secondary" onClick={addOption} style={{ marginBottom: 16 }}>
            + Add option
          </button>
        )}

        <button className="btn btn-block" disabled={loading}>
          {loading ? 'Creating…' : 'Create poll'}
        </button>
      </form>
    </div>
  )
}
