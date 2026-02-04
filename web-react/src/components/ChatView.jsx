import { motion } from 'framer-motion'
import { useStore } from '../stores/useStore'
import { useState, useRef, useEffect } from 'react'
import PasswordGate from './PasswordGate'
import './ChatView.css'

export default function ChatView() {
  return (
    <PasswordGate>
      <ChatContent />
    </PasswordGate>
  )
}

function ChatContent() {
  const chatMessages = useStore((state) => state.chatMessages)
  const chatLoading = useStore((state) => state.chatLoading)
  const sendChatMessage = useStore((state) => state.sendChatMessage)
  const clearChat = useStore((state) => state.clearChat)

  const [input, setInput] = useState('')
  const messagesEndRef = useRef(null)

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  useEffect(() => {
    scrollToBottom()
  }, [chatMessages])

  const handleSubmit = (e) => {
    e.preventDefault()
    if (!input.trim() || chatLoading) return
    sendChatMessage(input.trim())
    setInput('')
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit(e)
    }
  }

  return (
    <motion.div
      className="chat-view"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, y: 20 }}
    >
      <div className="chat-header">
        <h2>智能助手</h2>
        <p className="chat-subtitle">发送自然语言指令，让 ServerOwl 帮你管理服务器</p>
        {chatMessages.length > 0 && (
          <button className="clear-btn" onClick={clearChat}>清空对话</button>
        )}
      </div>

      <div className="chat-messages">
        {chatMessages.length === 0 ? (
          <div className="chat-empty">
            <div className="chat-empty-icon">🦉</div>
            <h3>欢迎使用 ServerOwl</h3>
            <p>你可以用自然语言发送指令，例如：</p>
            <div className="chat-suggestions">
              {[
                '查看所有容器状态',
                '分析一下系统负载',
                '查看磁盘使用情况',
                '检查 nginx 日志'
              ].map((suggestion, i) => (
                <button
                  key={i}
                  className="suggestion-btn"
                  onClick={() => {
                    setInput(suggestion)
                  }}
                >
                  {suggestion}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <>
            {chatMessages.map((msg, i) => (
              <motion.div
                key={i}
                className={`chat-message ${msg.role} ${msg.isError ? 'error' : ''}`}
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: i * 0.05 }}
              >
                <div className="message-avatar">
                  {msg.role === 'user' ? '👤' : '🦉'}
                </div>
                <div className="message-content">
                  <div className="message-text">{msg.content}</div>
                  <div className="message-meta">
                    <span className="message-time">
                      {new Date(msg.timestamp).toLocaleTimeString()}
                    </span>
                    {msg.duration && (
                      <span className="message-duration">耗时 {msg.duration}</span>
                    )}
                  </div>
                </div>
              </motion.div>
            ))}
            {chatLoading && (
              <motion.div
                className="chat-message assistant loading"
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
              >
                <div className="message-avatar">🦉</div>
                <div className="message-content">
                  <div className="typing-indicator">
                    <span></span>
                    <span></span>
                    <span></span>
                  </div>
                </div>
              </motion.div>
            )}
            <div ref={messagesEndRef} />
          </>
        )}
      </div>

      <form className="chat-input-form" onSubmit={handleSubmit}>
        <textarea
          className="chat-input"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入指令... (Enter 发送, Shift+Enter 换行)"
          disabled={chatLoading}
          rows={1}
        />
        <button
          type="submit"
          className="send-btn"
          disabled={!input.trim() || chatLoading}
        >
          {chatLoading ? '发送中...' : '发送'}
        </button>
      </form>
    </motion.div>
  )
}
