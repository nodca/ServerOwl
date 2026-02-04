import { motion, AnimatePresence } from 'framer-motion'
import { useStore } from '../stores/useStore'
import { useEffect, useState } from 'react'
import Navigation from './Navigation'
import NodesView from './NodesView'
import CommandsView from './CommandsView'
import LogsView from './LogsView'
import ChatView from './ChatView'
import SettingsView from './SettingsView'
import './Dashboard.css'

export default function Dashboard() {
  const nodes = useStore((state) => state.nodes)
  const selectedNode = useStore((state) => state.selectedNode)
  const events = useStore((state) => state.events)
  const selectNode = useStore((state) => state.selectNode)
  const loadNodes = useStore((state) => state.loadNodes)
  const loadDashboard = useStore((state) => state.loadDashboard)
  const dashboardData = useStore((state) => state.dashboardData)
  const [activeView, setActiveView] = useState('overview')

  // 加载数据
  useEffect(() => {
    loadNodes()
    loadDashboard()
    // 每30秒刷新一次
    const interval = setInterval(() => {
      loadNodes()
      loadDashboard()
    }, 30000)
    return () => clearInterval(interval)
  }, [loadNodes, loadDashboard])

  // ESC 键退出详情
  useEffect(() => {
    const handleKeyDown = (e) => {
      console.log('Key pressed:', e.key, 'selectedNode:', selectedNode)
      if (e.key === 'Escape' && selectedNode) {
        console.log('Closing detail panel')
        selectNode(null)
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [selectedNode, selectNode])

  const handleClose = () => {
    console.log('Close button clicked, current selectedNode:', selectedNode)
    console.log('selectNode function:', selectNode)
    selectNode(null)
    console.log('After calling selectNode(null)')
  }

  const stats = {
    total: nodes.size,
    online: Array.from(nodes.values()).filter(n => n.status === 'online').length,
    offline: Array.from(nodes.values()).filter(n => n.status === 'offline').length,
    warning: Array.from(nodes.values()).filter(n => n.metrics?.cpu > 80 || n.metrics?.memory > 80).length
  }

  const selectedNodeData = selectedNode ? nodes.get(selectedNode) : null

  return (
    <div className="dashboard">
      {/* 导航菜单 */}
      <Navigation activeView={activeView} onViewChange={setActiveView} />

      {/* 顶部导航 */}
      <motion.header
        className="header"
        initial={{ y: -100, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        transition={{ duration: 0.8, ease: 'easeOut' }}
      >
        <div className="logo">
          <div className="logo-icon">◉</div>
          <h1>OWL 深空观测站</h1>
        </div>
        <div className="status-bar">
          <div className="status-item">
            <span className="status-dot online"></span>
            <span>已连接</span>
          </div>
        </div>
      </motion.header>

      {/* 视图切换 */}
      <AnimatePresence mode="wait">
        {activeView === 'overview' && (
          <motion.div
            key="overview"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            {/* 统计卡片 */}
            <motion.div
              className="stats-grid"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ delay: 0.3, duration: 0.8 }}
            >
        {[
          { label: '总节点', value: stats.total, color: 'blue' },
          { label: '在线', value: stats.online, color: 'cyan' },
          { label: '离线', value: stats.offline, color: 'red' },
          { label: '告警', value: stats.warning, color: 'yellow' }
        ].map((stat, i) => (
          <motion.div
            key={stat.label}
            className={`stat-card ${stat.color}`}
            initial={{ scale: 0, rotate: -180 }}
            animate={{ scale: 1, rotate: 0 }}
            transition={{ delay: 0.5 + i * 0.1, type: 'spring', stiffness: 200 }}
          >
            <div className="stat-value">{stat.value}</div>
            <div className="stat-label">{stat.label}</div>
            <div className="stat-glow"></div>
          </motion.div>
        ))}
      </motion.div>

      {/* 节点详情 */}
      <AnimatePresence>
        {selectedNodeData && (
          <>
            {/* 遮罩层 - 点击退出 */}
            <motion.div
              className="detail-backdrop"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              onClick={handleClose}
            />

            <motion.div
              className="node-detail"
              initial={{ x: 400, opacity: 0 }}
              animate={{ x: 0, opacity: 1 }}
              exit={{ x: 400, opacity: 0 }}
              transition={{ type: 'spring', stiffness: 100 }}
            >
              <div className="detail-header">
                <h3>{selectedNodeData.hostname}</h3>
                <div className="detail-header-right">
                  <span className={`status-badge ${selectedNodeData.status}`}>
                    {selectedNodeData.status === 'online' ? '在线' : '离线'}
                  </span>
                  <button className="close-btn" onClick={handleClose}>
                    ✕
                  </button>
                </div>
              </div>

          <div className="metrics">
            {[
              { key: 'cpu', label: 'CPU 使用率' },
              { key: 'memory', label: '内存使用率' },
              { key: 'disk', label: '磁盘使用率' }
            ].map(metric => (
              <div key={metric.key} className="metric-item">
                <div className="metric-label">{metric.label}</div>
                <div className="metric-bar">
                  <motion.div
                    className="metric-fill"
                    initial={{ width: 0 }}
                    animate={{ width: `${selectedNodeData.metrics[metric.key]}%` }}
                    transition={{ duration: 1, ease: 'easeOut' }}
                  />
                </div>
                <div className="metric-value">{selectedNodeData.metrics[metric.key]}%</div>
              </div>
            ))}
          </div>
        </motion.div>
        </>
      )}
      </AnimatePresence>

      {/* 事件流 */}
      <motion.div
        className="events-panel"
        initial={{ y: 400, opacity: 0 }}
        animate={{ y: 0, opacity: 1 }}
        transition={{ delay: 0.6, duration: 0.8 }}
      >
        <h3>系统事件</h3>
        <div className="events-list">
          {events.length === 0 ? (
            <div className="events-empty">暂无事件记录</div>
          ) : (
            events.slice(0, 5).map((event, i) => (
              <motion.div
                key={i}
                className="event-item"
                initial={{ x: -50, opacity: 0 }}
                animate={{ x: 0, opacity: 1 }}
                transition={{ delay: i * 0.1 }}
              >
                <span className="event-time">
                  {new Date(event.timestamp).toLocaleTimeString()}
                </span>
                <span className="event-message">{event.message}</span>
              </motion.div>
            ))
          )}
        </div>
      </motion.div>

      {/* 提示文字 */}
      <motion.div
        className="hint"
        initial={{ opacity: 0 }}
        animate={{ opacity: 1 }}
        transition={{ delay: 1.5, duration: 1 }}
      >
        <p>◉ 拖动旋转 ◉ 滚轮缩放 ◉ 点击节点查看详情 ◉ ESC/点击空白退出 ◉</p>
      </motion.div>
          </motion.div>
        )}

        {activeView === 'nodes' && <NodesView key="nodes" />}

        {activeView === 'chat' && <ChatView key="chat" />}

        {activeView === 'commands' && <CommandsView key="commands" />}

        {activeView === 'logs' && <LogsView key="logs" />}

        {activeView === 'settings' && <SettingsView key="settings" />}
      </AnimatePresence>
    </div>
  )
}
