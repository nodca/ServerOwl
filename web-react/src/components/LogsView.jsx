import { motion } from 'framer-motion'
import { useStore } from '../stores/useStore'
import { useState, useEffect } from 'react'
import './LogsView.css'

export default function LogsView() {
  const actionLogs = useStore((state) => state.actionLogs)
  const logAnalysis = useStore((state) => state.logAnalysis)
  const loadActionLogs = useStore((state) => state.loadActionLogs)
  const loadLogAnalysis = useStore((state) => state.loadLogAnalysis)
  const [filterStatus, setFilterStatus] = useState('all')
  const [searchTerm, setSearchTerm] = useState('')

  useEffect(() => {
    loadActionLogs(100)
    loadLogAnalysis()
  }, [loadActionLogs, loadLogAnalysis])

  const filteredLogs = actionLogs.filter(log => {
    const matchesStatus = filterStatus === 'all' ||
      (filterStatus === 'success' && log.success) ||
      (filterStatus === 'error' && !log.success)
    const matchesSearch = log.tool_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (log.arguments && log.arguments.toLowerCase().includes(searchTerm.toLowerCase()))
    return matchesStatus && matchesSearch
  })

  const getStatusColor = (success) => {
    return success ? '#10b981' : '#ef4444'
  }

  const getStatusIcon = (success) => {
    return success ? '✓' : '✕'
  }

  const formatDuration = (ms) => {
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(2)}s`
  }

  return (
    <motion.div
      className="logs-view"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
    >
      <div className="view-header">
        <h2>操作日志</h2>
        <div className="view-controls">
          <div className="search-box">
            <span className="search-icon">🔍</span>
            <input
              type="text"
              placeholder="搜索工具名称..."
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
            />
          </div>
          <select
            className="filter-select"
            value={filterStatus}
            onChange={(e) => setFilterStatus(e.target.value)}
          >
            <option value="all">全部状态</option>
            <option value="success">成功</option>
            <option value="error">失败</option>
          </select>
          <button
            className="refresh-btn"
            onClick={() => {
              loadActionLogs(100)
              loadLogAnalysis()
            }}
          >
            刷新
          </button>
        </div>
      </div>

      <div className="logs-container">
        <div className="logs-timeline">
          {filteredLogs.length === 0 ? (
            <div className="logs-empty">
              {searchTerm || filterStatus !== 'all' ? '没有匹配的日志' : '暂无操作日志'}
            </div>
          ) : (
            filteredLogs.map((log, i) => (
              <motion.div
                key={log.id || i}
                className={`log-item ${log.success ? 'success' : 'error'}`}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: i * 0.02 }}
                style={{ borderLeftColor: getStatusColor(log.success) }}
              >
                <div className="log-icon" style={{ color: getStatusColor(log.success) }}>
                  {getStatusIcon(log.success)}
                </div>
                <div className="log-content">
                  <div className="log-header">
                    <span className="log-tool" style={{ color: getStatusColor(log.success) }}>
                      {log.tool_name}
                    </span>
                    <span className="log-duration">
                      {formatDuration(log.duration_ms)}
                    </span>
                    <span className="log-time">
                      {log.created_at
                        ? new Date(log.created_at).toLocaleString()
                        : '-'}
                    </span>
                  </div>
                  <div className="log-message">
                    {log.error_msg || (log.result ? log.result.substring(0, 100) : '执行成功')}
                  </div>
                  {log.session_id && (
                    <div className="log-meta">
                      会话: {log.session_id.substring(0, 16)}...
                    </div>
                  )}
                </div>
              </motion.div>
            ))
          )}
        </div>

        {/* 统计信息 */}
        <div className="logs-stats">
          <div className="stat-item">
            <span className="stat-label">总计</span>
            <span className="stat-value">{logAnalysis?.total_count || actionLogs.length}</span>
          </div>
          <div className="stat-item">
            <span className="stat-label">成功</span>
            <span className="stat-value success">
              {logAnalysis?.success_count || actionLogs.filter(l => l.success).length}
            </span>
          </div>
          <div className="stat-item">
            <span className="stat-label">失败</span>
            <span className="stat-value error">
              {logAnalysis?.failure_count || actionLogs.filter(l => !l.success).length}
            </span>
          </div>
          <div className="stat-item">
            <span className="stat-label">成功率</span>
            <span className="stat-value info">
              {logAnalysis?.success_rate ? `${logAnalysis.success_rate.toFixed(1)}%` : '-'}
            </span>
          </div>
        </div>
      </div>
    </motion.div>
  )
}
