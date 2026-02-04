import { useState } from 'react'
import { motion } from 'framer-motion'
import { useStore } from '../stores/useStore'
import './PasswordGate.css'

export default function PasswordGate({ children }) {
  const isAuthenticated = useStore((state) => state.isAuthenticated)
  const authenticate = useStore((state) => state.authenticate)
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')

  if (isAuthenticated) {
    return children
  }

  const handleSubmit = (e) => {
    e.preventDefault()
    if (authenticate(password)) {
      setError('')
    } else {
      setError('密码错误')
      setPassword('')
    }
  }

  return (
    <motion.div
      className="password-gate"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
    >
      <div className="password-box">
        <div className="password-icon">🔒</div>
        <h3>需要验证</h3>
        <p>此功能需要密码才能访问</p>
        <form onSubmit={handleSubmit}>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="请输入密码"
            autoFocus
          />
          {error && <div className="password-error">{error}</div>}
          <button type="submit">确认</button>
        </form>
      </div>
    </motion.div>
  )
}
