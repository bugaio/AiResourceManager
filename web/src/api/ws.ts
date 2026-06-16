/** WebSocket连接状态 */
type WsStatus = 'connecting' | 'connected' | 'disconnected'

/** 消息回调类型 */
type MessageHandler = (data: unknown) => void

/** WebSocket管理器 - 支持自动重连和心跳 */
class WebSocketManager {
  private ws: WebSocket | null = null
  private url: string = ''
  private status: WsStatus = 'disconnected'
  private reconnectAttempts: number = 0
  private maxReconnectDelay: number = 30000
  private baseDelay: number = 1000
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private messageHandlers: Set<MessageHandler> = new Set()

  /** 根据当前页面地址构建WebSocket URL */
  private buildUrl(): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    return `${protocol}//${host}/api/v1/ws`
  }

  /** 连接WebSocket服务 */
  connect(): void {
    if (this.ws && (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)) {
      return
    }

    this.url = this.buildUrl()
    this.status = 'connecting'
    this.ws = new WebSocket(this.url)

    this.ws.onopen = () => {
      this.status = 'connected'
      this.reconnectAttempts = 0
      this.startHeartbeat()
    }

    this.ws.onmessage = (event: MessageEvent) => {
      let data: unknown
      try {
        data = JSON.parse(event.data)
      } catch {
        data = event.data
      }

      // 心跳ping响应pong
      if (data && typeof data === 'object' && (data as Record<string, unknown>).type === 'ping') {
        this.ws?.send(JSON.stringify({ type: 'pong' }))
        return
      }

      // 分发消息给所有处理器
      this.messageHandlers.forEach((handler) => handler(data))
    }

    this.ws.onclose = () => {
      this.status = 'disconnected'
      this.stopHeartbeat()
      this.scheduleReconnect()
    }

    this.ws.onerror = () => {
      this.ws?.close()
    }
  }

  /** 断开WebSocket连接 */
  disconnect(): void {
    this.clearReconnectTimer()
    this.stopHeartbeat()
    if (this.ws) {
      this.ws.onclose = null
      this.ws.close()
      this.ws = null
    }
    this.status = 'disconnected'
    this.reconnectAttempts = 0
  }

  /** 计划自动重连 - 指数退避策略(1s→30s) */
  private scheduleReconnect(): void {
    this.clearReconnectTimer()
    const delay = Math.min(
      this.baseDelay * Math.pow(2, this.reconnectAttempts),
      this.maxReconnectDelay
    )
    this.reconnectAttempts++
    this.reconnectTimer = setTimeout(() => {
      this.connect()
    }, delay)
  }

  /** 清除重连定时器 */
  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  /** 启动心跳检测 - 每30秒发送一次 */
  private startHeartbeat(): void {
    this.stopHeartbeat()
    this.heartbeatTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        this.ws.send(JSON.stringify({ type: 'pong' }))
      }
    }, 30000)
  }

  /** 停止心跳检测 */
  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  /** 注册消息处理回调 */
  onMessage(handler: MessageHandler): void {
    this.messageHandlers.add(handler)
  }

  /** 移除消息处理回调 */
  offMessage(handler: MessageHandler): void {
    this.messageHandlers.delete(handler)
  }

  /** 发送消息 */
  send(data: unknown): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data))
    }
  }

  /** 获取当前连接状态 */
  getStatus(): WsStatus {
    return this.status
  }
}

/** 全局WebSocket管理器单例 */
const wsManager = new WebSocketManager()

export default wsManager
