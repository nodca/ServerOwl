import { motion } from 'framer-motion'
import { useStore } from '../stores/useStore'
import { useState } from 'react'
import { api } from '../services/api'
import './NodesView.css'

export default function NodesView() {
  const nodes = useStore((state) => state.nodes)
  const selectNode = useStore((state) => state.selectNode)
  const [searchTerm, setSearchTerm] = useState('')
  const [filterStatus, setFilterStatus] = useState('all')
  const [refreshing, setRefreshing] = useState(null)

  const handleRefreshEnv = async (nodeId) => {
    setRefreshing(nodeId)
    try {
      const res = await api.environment.refresh(nodeId)
      if (res.success) {
        alert(res.message || '已发送刷新请求')
      } else {
        alert('刷新失败: ' + (res.error || '未知错误'))
      }
    } catch (err) {
      alert('刷新失败: ' + err.message)
    } finally {
      setRefreshing(null)
    }
  }

  const nodesList = Array.from(nodes.values()).filter(node => {
    const matchesSearch = node.hostname.toLowerCase().includes(searchTerm.toLowerCase())
    const matchesFilter = filterStatus === 'all' || node.status === filterStatus
    return matchesSearch && matchesFilter
  })

  return (
    <motion.div
      className="nodes-view"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
      transition={{ duration: 0.5 }}
    >
      <div className="view-header">
        <h2>节点管理</h2>
        <div className="view-controls">
          <div className="search-box">
            <span className="search-icon">🔍</span>
            <input
              type="text"
              placeholder="搜索节点..."
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
            <option value="online">在线</option>
            <option value="offline">离线</option>
          </select>
        </div>
      </div>

      <div className="nodes-table-container">
        <table className="nodes-table">
          <thead>
            <tr>
              <th>状态</th>
              <th>节点名称</th>
              <th>类型</th>
              <th>CPU</th>
              <th>内存</th>
              <th>磁盘</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {nodesList.map((node, i) => (
              <motion.tr
                key={node.id}
                initial={{ opacity: 0, x: -20 }}
                animate={{ opacity: 1, x: 0 }}
                transition={{ delay: i * 0.05 }}
                whileHover={{ backgroundColor: 'rgba(217, 70, 239, 0.1)' }}
              >
                <td>
                  <span className={`status-indicator ${node.status}`}>
                    {node.status === 'online' ? '●' : '○'}
                  </span>
                </td>
                <td className="node-name">
                  {node.hostname}
                  {node.isMaster && <span className="master-badge">MASTER</span>}
                </td>
                <td>{node.isMaster ? '主控节点' : '工作节点'}</td>
                <td>
                  <div className="metric-bar-mini">
                    <div
                      className="metric-fill-mini cpu"
                      style={{ width: `${node.metrics.cpu}%` }}
                    />
                    <span className="metric-text">{node.metrics.cpu}%</span>
                  </div>
                </td>
                <td>
                  <div className="metric-bar-mini">
                    <div
                      className="metric-fill-mini memory"
                      style={{ width: `${node.metrics.memory}%` }}
                    />
                    <span className="metric-text">{node.metrics.memory}%</span>
                  </div>
                </td>
                <td>
                  <div className="metric-bar-mini">
                    <div
                      className="metric-fill-mini disk"
                      style={{ width: `${node.metrics.disk}%` }}
                    />
                    <span className="metric-text">{node.metrics.disk}%</span>
                  </div>
                </td>
                <td>
                  <button
                    className="action-btn"
                    onClick={() => selectNode(node.id)}
                  >
                    详情
                  </button>
                  <button
                    className="action-btn refresh-btn"
                    onClick={() => handleRefreshEnv(node.id)}
                    disabled={refreshing === node.id}
                    title="刷新环境配置"
                  >
                    {refreshing === node.id ? '...' : '🔄'}
                  </button>
                </td>
              </motion.tr>
            ))}
          </tbody>
        </table>
      </div>
    </motion.div>
  )
}
