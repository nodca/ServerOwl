import { motion } from 'framer-motion'
import { useState, useEffect } from 'react'
import { api } from '../services/api'
import PasswordGate from './PasswordGate'
import './SettingsView.css'

export default function SettingsView() {
  return (
    <PasswordGate>
      <SettingsContent />
    </PasswordGate>
  )
}

function SettingsContent() {
  const [allEnvs, setAllEnvs] = useState({})
  const [selectedNode, setSelectedNode] = useState('local')
  const [envContent, setEnvContent] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [message, setMessage] = useState(null)

  useEffect(() => {
    loadEnvironment()
  }, [])

  useEffect(() => {
    // 当选择的节点变化时，更新编辑器内容
    if (allEnvs[selectedNode]) {
      setEnvContent(formatYaml(allEnvs[selectedNode]))
    }
  }, [selectedNode, allEnvs])

  const loadEnvironment = async () => {
    setLoading(true)
    try {
      const res = await api.environment.getAll()
      if (res.success && res.data) {
        setAllEnvs(res.data)
        // 选择第一个节点
        const nodes = Object.keys(res.data)
        if (nodes.length > 0) {
          setSelectedNode(nodes[0])
          setEnvContent(formatYaml(res.data[nodes[0]]))
        }
      } else {
        setMessage({ type: 'error', text: '加载失败: ' + (res.error || '未知错误') })
      }
    } catch (err) {
      setMessage({ type: 'error', text: '加载失败: ' + err.message })
    } finally {
      setLoading(false)
    }
  }

  const handleRefresh = async () => {
    setRefreshing(true)
    setMessage(null)
    try {
      // 刷新当前选中的节点
      const isLocal = selectedNode === 'local' || Object.keys(allEnvs).length <= 1
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

  // 简单的对象转 YAML 格式
  const formatYaml = (obj, indent = 0) => {
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

  const nodeList = Object.keys(allEnvs)

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
              disabled={saving || selectedNode !== 'local'}
              title={selectedNode !== 'local' ? '只能编辑本地节点配置' : ''}
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
                className={`node-tab ${selectedNode === nodeId ? 'active' : ''}`}
                onClick={() => setSelectedNode(nodeId)}
              >
                {allEnvs[nodeId]?.host?.hostname || nodeId}
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
            readOnly={selectedNode !== 'local' && nodeList.length > 1}
          />
        )}
      </div>
    </motion.div>
  )
}
