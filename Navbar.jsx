import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext.jsx'

export default function Navbar() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  return (
    <div className="navbar">
      <Link to="/" className="brand">Live<span>Poll</span></Link>
      <div className="navbar-links">
        {user ? (
          <>
            <Link to="/dashboard">Dashboard</Link>
            <Link to="/create">New Poll</Link>
            <button
              className="btn btn-secondary"
              onClick={() => { logout(); navigate('/') }}
            >
              Log out
            </button>
          </>
        ) : (
          <>
            <Link to="/login">Log in</Link>
            <Link to="/signup" className="btn">Sign up</Link>
          </>
        )}
      </div>
    </div>
  )
}
