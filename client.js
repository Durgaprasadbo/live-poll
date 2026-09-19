import axios from 'axios'

export const API_BASE = import.meta.env.VITE_API_URL || 'http://localhost:8080'
export const WS_BASE = API_BASE.replace(/^http/, 'ws')

const client = axios.create({ baseURL: `${API_BASE}/api` })

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

// Every browser gets a stable anonymous voter id, used server-side to
// stop the same browser voting twice on one poll.
export function getVoterId() {
  let id = localStorage.getItem('voter_id')
  if (!id) {
    id = crypto.randomUUID()
    localStorage.setItem('voter_id', id)
  }
  return id
}

export default client
