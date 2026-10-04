import { SwarmEvent } from '../types';

type EventHandler = (event: SwarmEvent) => void;
type StatusHandler = (connected: boolean) => void;

class WebSocketClient {
  private socket: WebSocket | null = null;
  private url: string;
  private eventHandlers: Set<EventHandler> = new Set();
  private statusHandlers: Set<StatusHandler> = new Set();
  private reconnectTimer: any = null;
  private isConnected: boolean = false;
  private useProxy: boolean = false;

  constructor(url?: string) {
    this.url = url || '';
    (window as any).__wsClient = this;
  }

  private getUrl(): string {
    if (this.url) return this.url;
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    if (this.useProxy) {
      return `${proto}//${window.location.host}/ws`;
    }
    const host = window.location.hostname || '127.0.0.1';
    return `${proto}//${host}:8080/ws`;
  }

  public connect() {
    if (this.socket && (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING)) {
      return;
    }

    try {
      const targetUrl = this.getUrl();
      this.socket = new WebSocket(targetUrl);

      this.socket.onopen = () => {
        this.isConnected = true;
        this.notifyStatus(true);
        if (this.reconnectTimer) {
          clearTimeout(this.reconnectTimer);
          this.reconnectTimer = null;
        }
      };

      this.socket.onmessage = (event) => {
        try {
          const parsed = JSON.parse(event.data);
          this.eventHandlers.forEach(handler => handler(parsed));
        } catch (err) {
          console.error("Failed to parse websocket event:", err);
        }
      };

      this.socket.onclose = () => {
        this.isConnected = false;
        this.notifyStatus(false);
        this.useProxy = !this.useProxy;
        this.scheduleReconnect();
      };

      this.socket.onerror = () => {
        this.socket?.close();
      };
    } catch (e) {
      this.useProxy = !this.useProxy;
      this.scheduleReconnect();
    }
  }

  private scheduleReconnect() {
    if (!this.reconnectTimer) {
      this.reconnectTimer = setTimeout(() => {
        this.reconnectTimer = null;
        this.connect();
      }, 2000);
    }
  }

  public onEvent(handler: EventHandler): () => void {
    this.eventHandlers.add(handler);
    return () => this.eventHandlers.delete(handler);
  }

  public onStatus(handler: StatusHandler): () => void {
    this.statusHandlers.add(handler);
    handler(this.isConnected);
    return () => this.statusHandlers.delete(handler);
  }

  private notifyStatus(status: boolean) {
    this.statusHandlers.forEach(h => h(status));
  }

  public send(action: string, project_id: string, payload?: any, user_prompt?: string) {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({
        action,
        project_id,
        user_prompt,
        payload,
      }));
    } else {
      console.warn("WebSocket not open. Unable to send action:", action);
    }
  }
}

export const wsClient = new WebSocketClient();
