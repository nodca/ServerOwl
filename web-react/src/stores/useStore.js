import { create } from 'zustand'
import { api } from '../services/api'

const AUTH_PASSWORD = '9638527410w'
const AUTH_KEY = 'serverowl_auth'

export const useStore = create((set, get) => ({
  // 认证状态
  isAuthenticated: localStorage.getItem(AUTH_KEY) === 'true',

  authenticate: (password) => {
    if (password === AUTH_PASSWORD) {
      localStorage.setItem(AUTH_KEY, 'true')
      set({ isAuthenticated: true })
      return true
    }
    return false
  },

  logout: () => {
    localStorage.removeItem(AUTH_KEY)
    set({ isAuthenticated: false })
  },

  // 节点数据 - 初始为空，从API加载
  nodes: new Map(),
  selectedNode: null,
  events: [],

  // Dashboard 数据
  dashboardData: null,

  // 操作日志
  actionLogs: [],
  logAnalysis: null,

  // 对话相关
  chatMessages: [],
  chatSessionId: null,
  chatLoading: false,

  // 加载节点数据
  loadNodes: async () => {
    try {
      const res = await api.cluster.getNodes()
      if (res.success && res.data) {
        const nodesMap = new Map()
        res.data.forEach(node => {
          // 从 API 获取指标数据（Agent 上报的）
          const metrics = node.metrics ? {
            cpu: Math.round((node.metrics.cpu || 0) * 10) / 10,
            memory: Math.round((node.metrics.memory || 0) * 10) / 10,
            disk: Math.round((node.metrics.disk || 0) * 10) / 10
          } : { cpu: 0, memory: 0, disk: 0 }

          nodesMap.set(node.id, {
            id: node.id,
            hostname: node.hostname || node.name || node.host,
            status: node.status,
            isMaster: node.tags?.includes('master') || false,
            metrics,
            host: node.host,
            port: node.port,
            tags: node.tags,
            lastHeartbeat: node.last_heartbeat
          })
        })
        set({ nodes: nodesMap })
      }
    } catch (err) {
      console.error('加载节点失败:', err)
    }
  },

  // 加载操作日志
  loadActionLogs: async (limit = 50) => {
    try {
      const res = await api.logs.actions(limit)
      if (res.success && res.data) {
        set({ actionLogs: res.data })
      }
    } catch (err) {
      console.error('加载操作日志失败:', err)
    }
  },

  // 加载日志分析
  loadLogAnalysis: async () => {
    try {
      const res = await api.logs.analyze()
      if (res.success && res.data) {
        set({ logAnalysis: res.data })
      }
    } catch (err) {
      console.error('加载日志分析失败:', err)
    }
  },

  // 加载 Dashboard 数据
  loadDashboard: async () => {
    try {
      const res = await api.dashboard()
      if (res.success && res.data) {
        set({ dashboardData: res.data })
      }
    } catch (err) {
      console.error('加载Dashboard失败:', err)
    }
  },

  selectNode: (nodeId) => {
    set({ selectedNode: nodeId })
  },

  updateNode: (nodeId, data) => set((state) => {
    const newNodes = new Map(state.nodes)
    newNodes.set(nodeId, { ...newNodes.get(nodeId), ...data })
    return { nodes: newNodes }
  }),

  addEvent: (event) => set((state) => ({
    events: [{ ...event, timestamp: Date.now() }, ...state.events].slice(0, 100)
  })),

  // 发送对话消息
  sendChatMessage: async (message) => {
    const { chatSessionId, chatMessages } = get()

    // 添加用户消息
    const userMsg = { role: 'user', content: message, timestamp: Date.now() }
    set({
      chatMessages: [...chatMessages, userMsg],
      chatLoading: true
    })

    try {
      const res = await api.chat(message, chatSessionId)
      if (res.success) {
        const assistantMsg = {
          role: 'assistant',
          content: res.message,
          timestamp: Date.now(),
          duration: res.duration
        }
        set((state) => ({
          chatMessages: [...state.chatMessages, assistantMsg],
          chatSessionId: res.session_id,
          chatLoading: false
        }))
      } else {
        const errorMsg = {
          role: 'assistant',
          content: `错误: ${res.error}`,
          timestamp: Date.now(),
          isError: true
        }
        set((state) => ({
          chatMessages: [...state.chatMessages, errorMsg],
          chatLoading: false
        }))
      }
    } catch (err) {
      const errorMsg = {
        role: 'assistant',
        content: `请求失败: ${err.message}`,
        timestamp: Date.now(),
        isError: true
      }
      set((state) => ({
        chatMessages: [...state.chatMessages, errorMsg],
        chatLoading: false
      }))
    }
  },

  clearChat: () => set({ chatMessages: [], chatSessionId: null })
}))
