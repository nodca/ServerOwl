import { motion } from 'framer-motion'
import { useState, useEffect, useCallback } from 'react'
import { api } from '../services/api'
import PasswordGate from './PasswordGate'
import './SettingsView.css'

// 简单的对象转 YAML 格式（移到组件外部避免依赖问题）
const formatYaml = (obj, indent = 0) => {
  if (!obj || typeof obj !== 'object') return ''
  const spaces = '  '.repeat(indent)
  let result = ''

  for (const [key, value] of Object.entries(obj)) {
    if (value === null || value === undefined) {
      result += `${spaces}${key}:\n`
    } else if (typeof value === 'object' && !Array.isArray(value)) {
      result += `${spaces}${key}:\n${formatYaml(value, indent + 1)}`
    } else if (Array.isArray(value)) {
      result += `${spaces}${key}:\n`
      value.forEach(item => {
        if (typeof item === 'object') {
          result += `${spaces}  -\n`
          for (const [k, v] of Object.entries(item)) {
            result += `${spaces}    ${k}: ${v}\n`
          }
        } else {
          result += `${spaces}  - ${item}\n`
        }
      })
    } else {
      result += `${spaces}${key}: ${value}\n`
    }
  }

  return result
}

// 格式化更新时间
const formatTime = (timeStr) => {
  if (!timeStr) return ''
  // 如果是完整时间格式，只取时间部分
  if (timeStr.includes(' ')) {
    return timeStr.split(' ')[1]?.substring(0, 5) || timeStr
  }
  return timeStr
}

export default function SettingsView() {
  return (
    <PasswordGate>
      <SettingsContent />
    </PasswordGate>
  )
}

function SettingsContent() {
  const [allEnvs, setAllEnvs] = useState({})
  const [localNodeId, setLocalNodeId] = useState(null)
  const [selectedNode, setSelectedNode] = useState(null)
  const [envContent, setEnvContent] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [message, setMessage] = useState(null)

  const loadEnvironment = useCallback(async () => {
    setLoading(true)
    try {
      const res = await api.environment.getAll()
      if (res.success && res.data) {
        setAllEnvs(res.data)
        // 本地节点 ID 由后端返回（第一个节点通常是本地）
        const nodes = Object.keys(res.data)
        if (nodes.length > 0) {
          // 后端返回的 localNodeId，如果没有则用第一个
          const localId = res.localNodeId || nodes[0]
          setLocalNodeId(localId)
          // 默认选中本地节点
          if (!selectedNode || !nodes.includes(selectedNode)) {
            setSelectedNode(localId)
            setEnvContent(formatYaml(res.data[localId]))
          }
        }
      } else {
        setMessage({ type: 'error', text: '加载失败: ' + (res.error || '未知错误') })
      }
    } catch (err) {
      setMessage({ type: 'error', text: '加载失败: ' + err.message })
    } finally {
      setLoading(false)
    }
  }, [selectedNode])

  useEffect(() => {
    loadEnvironment()
  }, [])

  useEffect(() => {
    // 当选择的节点变化时，更新编辑器内容
    if (selectedNode && allEnvs[selectedNode]) {
      setEnvContent(formatYaml(allEnvs[selectedNode]))
    }
  }, [selectedNode, allEnvs])

  const isLocalNode = useCallback((nodeId) => {
    // 判断是否为本地节点
    if (!nodeId) return false
    if (localNodeId) return nodeId === localNodeId
    // 如果没有 localNodeId，检查是否只有一个节点
    return Object.keys(allEnvs).length <= 1
  }, [localNodeId, allEnvs])

  const handleRefresh = async () => {
    setRefreshing(true)
    setMessage(null)
    try {
      // 刷新当前选中的节点
      const isLocal = isLocalNode(selectedNode)
      const res = await api.environment.refresh(isLocal ? null : selectedNode)
      if (res.success) {
        setMessage({ type: 'success', text: res.message || '已发送刷新请求' })
        // 重新加载所有环境
        setTimeout(() => loadEnvironment(), 1000)
      } else {
        setMessage({ type: 'error', text: '刷新失败: ' + (res.error || '未知错误') })
      }
    } catch (err) {
      setMessage({ type: 'error', text: '刷新失败: ' + err.message })
    } finally {
      setRefreshing(false)
    }
  }

  const handleRefreshAll = async () => {
    setRefreshing(true)
    setMessage(null)
    try {
      const res = await api.environment.refreshAll()
      if (res.success) {
        setMessage({ type: 'success', text: res.message || '已向所有节点发送刷新请求' })
        setTimeout(() => loadEnvironment(), 2000)
      } else {
        setMessage({ type: 'error', text: '刷新失败: ' + (res.error || '未知错误') })
      }
    } catch (err) {
      setMessage({ type: 'error', text: '刷新失败: ' + err.message })
    } finally {
      setRefreshing(false)
    }
  }

  const handleSave = async () => {
    setSaving(true)
    setMessage(null)
    try {
      const res = await api.environment.save(envContent)
      if (res.success) {
        setMessage({ type: 'success', text: '保存成功' })
      } else {
        setMessage({ type: 'error', text: '保存失败: ' + (res.error || '未知错误') })
      }
    } catch (err) {
      setMessage({ type: 'error', text: '保存失败: ' + err.message })
    } finally {
      setSaving(false)
    }
  }

  const nodeList = Object.keys(allEnvs)
  const canEdit = isLocalNode(selectedNode)

  return (
    <motion.div
      className="settings-view"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: 20 }}
    >
      <div className="settings-header">
        <h2>系统设置</h2>
      </div>

      <div className="settings-section">
        <div className="section-header">
          <h3>环境配置</h3>
          <div className="section-actions">
            {nodeList.length > 1 && (
              <button
                className="action-btn"
                onClick={handleRefreshAll}
                disabled={refreshing}
              >
                {refreshing ? '刷新中...' : '🔄 刷新全部'}
              </button>
            )}
            <button
              className="action-btn"
              onClick={handleRefresh}
              disabled={refreshing}
            >
              {refreshing ? '刷新中...' : '🔄 重新扫描'}
            </button>
            <button
              className="action-btn primary"
              onClick={handleSave}
              disabled={saving || !canEdit}
              title={!canEdit ? '只能编辑本地节点配置' : ''}
            >
              {saving ? '保存中...' : '💾 保存'}
            </button>
          </div>
        </div>

        <p className="section-desc">
          环境配置包含服务器上的容器、数据库、代理等信息。Agent 会使用这些信息来回答问题。
          {nodeList.length > 1 && ' 选择节点查看对应的环境配置。'}
        </p>

        {nodeList.length > 1 && (
          <div className="node-tabs">
            {nodeList.map(nodeId => (
              <button
                key={nodeId}
                className={`node-tab ${selectedNode === nodeId ? 'active' : ''} ${isLocalNode(nodeId) ? 'local' : ''}`}
                onClick={() => setSelectedNode(nodeId)}
              >
                <span className="node-name">
                  {allEnvs[nodeId]?.host?.hostname || nodeId}
                  {isLocalNode(nodeId) && <span className="local-badge">本地</span>}
                </span>
                {allEnvs[nodeId]?.updated_at && (
                  <span className="node-time">{formatTime(allEnvs[nodeId].updated_at)}</span>
                )}
              </button>
            ))}
          </div>
        )}

        {message && (
          <div className={`message ${message.type}`}>
            {message.text}
          </div>
        )}

        {loading ? (
          <div className="loading">加载中...</div>
        ) : (
          <textarea
            className="env-editor"
            value={envContent}
            onChange={(e) => setEnvContent(e.target.value)}
            spellCheck={false}
            readOnly={!canEdit}
          />
        )}
      </div>
    </motion.div>
  )
}
