// API 服务层
const API_BASE = '/api/v1'

export const api = {
  // 健康检查
  health: async () => {
    const res = await fetch(`${API_BASE}/dashboard/health`)
    return res.json()
  },

  // Dashboard 数据
  dashboard: async () => {
    const res = await fetch(`${API_BASE}/dashboard/stats`)
    return res.json()
  },

  // 集群节点
  cluster: {
    getNodes: async () => {
      const res = await fetch(`${API_BASE}/cluster/nodes`)
      return res.json()
    },
    execute: async (nodeId, command) => {
      const res = await fetch(`${API_BASE}/cluster/execute`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ node_id: nodeId, command })
      })
      return res.json()
    }
  },

  // 日志
  logs: {
    actions: async (limit = 50) => {
      const res = await fetch(`${API_BASE}/logs/actions?limit=${limit}`)
      return res.json()
    },
    analyze: async () => {
      const res = await fetch(`${API_BASE}/logs/analyze`)
      return res.json()
    }
  },

  // 技能管理
  skills: {
    list: async () => {
      const res = await fetch(`${API_BASE}/skills`)
      return res.json()
    },
    execute: async (skillName, params) => {
      const res = await fetch(`${API_BASE}/skills/execute`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ skillName, params })
      })
      return res.json()
    }
  },

  // 任务管理
  tasks: {
    list: async () => {
      const res = await fetch(`${API_BASE}/tasks`)
      return res.json()
    }
  },

  // 记忆系统
  memory: {
    episodes: async () => {
      const res = await fetch(`${API_BASE}/memory/episodes`)
      return res.json()
    },
    knowledge: async () => {
      const res = await fetch(`${API_BASE}/memory/knowledge`)
      return res.json()
    }
  },

  // 环境配置
  environment: {
    get: async () => {
      const res = await fetch(`${API_BASE}/environment`)
      return res.json()
    },
    getAll: async () => {
      const res = await fetch(`${API_BASE}/environment?all=true`)
      return res.json()
    },
    refresh: async (nodeId = null) => {
      const url = nodeId
        ? `${API_BASE}/environment/refresh?node=${nodeId}`
        : `${API_BASE}/environment/refresh`
      const res = await fetch(url, {
        method: 'POST'
      })
      return res.json()
    },
    refreshAll: async () => {
      const res = await fetch(`${API_BASE}/environment/refresh?all=true`, {
        method: 'POST'
      })
      return res.json()
    },
    save: async (content) => {
      const res = await fetch(`${API_BASE}/environment`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content })
      })
      return res.json()
    }
  },

  // 对话接口
  chat: async (message, sessionId = null) => {
    const controller = new AbortController()
    const timeoutId = setTimeout(() => controller.abort(), 120000) // 2分钟超时

    try {
      const res = await fetch(`${API_BASE}/chat`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ message, session_id: sessionId }),
        signal: controller.signal
      })
      clearTimeout(timeoutId)

      if (!res.ok) {
        return { success: false, error: `HTTP ${res.status}` }
      }

      const text = await res.text()
      if (!text) {
        return { success: false, error: '服务器返回空响应' }
      }

      return JSON.parse(text)
    } catch (err) {
      clearTimeout(timeoutId)
      if (err.name === 'AbortError') {
        return { success: false, error: '请求超时，请稍后重试' }
      }
      throw err
    }
  }
}
