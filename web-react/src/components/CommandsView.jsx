import { motion } from 'framer-motion'
import { useStore } from '../stores/useStore'
import { useState } from 'react'
import { api } from '../services/api'
import PasswordGate from './PasswordGate'
import './CommandsView.css'

export default function CommandsView() {
  return (
    <PasswordGate>
      <CommandsContent />
    </PasswordGate>
  )
}

function CommandsContent() {
  const nodes = useStore((state) => state.nodes)
  const [selectedNode, setSelectedNode] = useState('')
  const [commandType, setCommandType] = useState('shell')
  const [command, setCommand] = useState('')
  const [executing, setExecuting] = useState(false)
  const [result, setResult] = useState(null)
  const [history, setHistory] = useState([])

  const handleExecute = async () => {
    if (!selectedNode || !command) {
      alert('请选择节点并输入命令')
      return
    }

    setExecuting(true)
    setResult(null)

    try {
      const res = await api.cluster.execute(selectedNode, command)
      setResult(res)

      // 添加到历史记录
      setHistory([
        {
          id: Date.now(),
          node: nodes.get(selectedNode)?.hostname || selectedNode,
          command,
          result: res,
          timestamp: new Date()
        },
        ...history
      ].slice(0, 10))

      setCommand('')
    } catch (error) {
      setResult({ error: error.message })
    } finally {
      setExecuting(false)
    }
  }

  return (
    <motion.div
      className="commands-view"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: -20 }}
    >
      <div className="view-header">
        <h2>命令中心</h2>
        <div className="status-indicator">
          <span className="pulse-dot"></span>
          <span>远程执行就绪</span>
        </div>
      </div>

      <div className="commands-layout">
        {/* 左侧：命令输入 */}
        <div className="command-panel">
          <div className="panel-section">
            <label className="form-label">目标节点</label>
            <select
              className="form-select"
              value={selectedNode}
              onChange={(e) => setSelectedNode(e.target.value)}
            >
              <option value="">选择节点...</option>
              {Array.from(nodes.values())
                .filter(n => n.status === 'online')
                .map(node => (
                  <option key={node.id} value={node.id}>
                    {node.hostname} {node.isMaster ? '(Master)' : ''}
                  </option>
                ))}
            </select>
          </div>

          <div className="panel-section">
            <label className="form-label">命令类型</label>
            <div className="radio-group">
              <label className={`radio-option ${commandType === 'shell' ? 'active' : ''}`}>
                <input
                  type="radio"
                  value="shell"
                  checked={commandType === 'shell'}
                  onChange={(e) => setCommandType(e.target.value)}
                />
                <span>Shell 命令</span>
              </label>
              <label className={`radio-option ${commandType === 'skill' ? 'active' : ''}`}>
                <input
                  type="radio"
                  value="skill"
                  checked={commandType === 'skill'}
                  onChange={(e) => setCommandType(e.target.value)}
                />
                <span>技能调用</span>
              </label>
            </div>
          </div>

          <div className="panel-section">
            <label className="form-label">命令内容</label>
            <textarea
              className="command-input"
              placeholder={commandType === 'shell' ? '例如: ls -la' : '例如: system_healthcheck'}
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              rows={6}
            />
          </div>

          <button
            className="execute-btn"
            onClick={handleExecute}
            disabled={executing || !selectedNode || !command}
          >
            {executing ? (
              <>
                <span className="spinner"></span>
                执行中...
              </>
            ) : (
              <>
                <span className="icon">⚡</span>
                执行命令
              </>
            )}
          </button>

          {/* 执行结果 */}
          {result && (
            <motion.div
              className="result-panel"
              initial={{ opacity: 0, y: 10 }}
              animate={{ opacity: 1, y: 0 }}
            >
              <div className="result-header">
                <span>执行结果</span>
                <span className={result.error ? 'status-error' : 'status-success'}>
                  {result.error ? '失败' : '成功'}
                </span>
              </div>
              <pre className="result-content">
                {result.error || result.message || JSON.stringify(result, null, 2)}
              </pre>
            </motion.div>
          )}
        </div>

        {/* 右侧：执行历史 */}
        <div className="history-panel">
          <h3 className="panel-title">执行历史</h3>
          <div className="history-list">
            {history.length === 0 ? (
              <div className="history-empty">暂无执行记录</div>
            ) : (
              history.map((item) => (
                <motion.div
                  key={item.id}
                  className="history-item"
                  initial={{ opacity: 0, x: 20 }}
                  animate={{ opacity: 1, x: 0 }}
                >
                  <div className="history-header">
                    <span className="history-node">{item.node}</span>
                    <span className="history-time">
                      {item.timestamp.toLocaleTimeString()}
                    </span>
                  </div>
                  <div className="history-command">{item.command}</div>
                  <div className={`history-status ${item.result.error ? 'error' : 'success'}`}>
                    {item.result.error ? '失败' : '成功'}
                  </div>
                </motion.div>
              ))
            )}
          </div>
        </div>
      </div>
    </motion.div>
  )
}
