import { motion } from 'framer-motion'
import { useState } from 'react'
import './Navigation.css'

const navItems = [
  { id: 'overview', label: '概览', icon: '◉' },
  { id: 'chat', label: '对话', icon: '💬' },
  { id: 'nodes', label: '节点', icon: '⬡' },
  { id: 'commands', label: '命令', icon: '⚡' },
  { id: 'logs', label: '日志', icon: '≡' },
  { id: 'settings', label: '设置', icon: '⚙' }
]

export default function Navigation({ activeView, onViewChange }) {
  const isOverview = activeView === 'overview'

  return (
    <motion.nav
      className={`navigation ${isOverview ? 'compact' : ''}`}
      initial={{ x: -100, opacity: 0 }}
      animate={{
        x: 0,
        opacity: isOverview ? 0.3 : 1,
        width: isOverview ? '60px' : '80px'
      }}
      transition={{ duration: 0.3 }}
      whileHover={{ opacity: 1 }}
    >
      <div className="nav-items">
        {navItems.map((item, i) => (
          <motion.button
            key={item.id}
            className={`nav-item ${activeView === item.id ? 'active' : ''}`}
            onClick={() => onViewChange(item.id)}
            initial={{ x: -50, opacity: 0 }}
            animate={{ x: 0, opacity: 1 }}
            transition={{ delay: 0.3 + i * 0.1 }}
            whileHover={{ scale: 1.05, x: 10 }}
            whileTap={{ scale: 0.95 }}
          >
            <span className="nav-icon">{item.icon}</span>
            <span className="nav-label">{item.label}</span>
            {activeView === item.id && (
              <motion.div
                className="nav-indicator"
                layoutId="activeIndicator"
                transition={{ type: 'spring', stiffness: 300, damping: 30 }}
              />
            )}
          </motion.button>
        ))}
      </div>

      {/* 装饰性扫描线 */}
      <div className="scan-line" />
    </motion.nav>
  )
}
