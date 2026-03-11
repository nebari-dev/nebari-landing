type WebSocketMessageHandler<TMessage = unknown> = (message: TMessage) => void;
type WebSocketEventHandler = () => void;
type WebSocketErrorHandler = (event: Event) => void;

export type CreateWebSocketClientOptions<TMessage = unknown> = {
  path: string;
  protocols?: string | string[];
  parseMessage?: (raw: string) => TMessage;
  onMessage?: WebSocketMessageHandler<TMessage>;
  onOpen?: WebSocketEventHandler;
  onClose?: WebSocketEventHandler;
  onError?: WebSocketErrorHandler;
  reconnect?: boolean;
  reconnectDelayMs?: number;
  queryParams?: Record<string, string | number | boolean | undefined>;
};

export type WebSocketClient = {
  connect: () => void;
  disconnect: () => void;
  send: (data: string) => void;
  isConnected: () => boolean;
  getSocket: () => WebSocket | null;
};

function buildWebSocketUrl(
  path: string,
  queryParams?: Record<string, string | number | boolean | undefined>
): string {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  const url = new URL(`${protocol}//${window.location.host}${normalizedPath}`);

  if (queryParams) {
    Object.entries(queryParams).forEach(([key, value]) => {
      if (value !== undefined) {
        url.searchParams.set(key, String(value));
      }
    });
  }

  return url.toString();
}

export function createWebSocketClient<TMessage = unknown>(
  options: CreateWebSocketClientOptions<TMessage>
): WebSocketClient{
  const {
    path,
    protocols,
    parseMessage = (raw) => JSON.parse(raw) as TMessage,
    onMessage,
    onOpen,
    onClose,
    onError,
    reconnect = true,
    reconnectDelayMs = 3000,
    queryParams,
  } = options;

  let socket: WebSocket | null = null;
  let reconnectTimer: number | null = null;
  let manuallyClosed = false;

  const clearReconnectTimer = () => {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  };

  const connect = () => {
    clearReconnectTimer();

    const url = buildWebSocketUrl(path, queryParams);
    socket = protocols ? new WebSocket(url, protocols) : new WebSocket(url);

    socket.addEventListener("open", () => {
      onOpen?.();
    });

    socket.addEventListener("message", (event) => {
      try {
        const parsed = parseMessage(event.data);
        onMessage?.(parsed);
      } catch (error) {
        console.error("Failed to parse websocket message", error, event.data);
      }
    });

    socket.addEventListener("close", () => {
      onClose?.();
      socket = null;

      if (!manuallyClosed && reconnect) {
        reconnectTimer = window.setTimeout(() => {
          connect();
        }, reconnectDelayMs);
      }
    });

    socket.addEventListener("error", (event) => {
      onError?.(event);
    });
  };

  const disconnect = () => {
    manuallyClosed = true;
    clearReconnectTimer();
    socket?.close();
    socket = null;
  };

  const send = (data: string) => {
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      throw new Error("WebSocket is not connected");
    }

    socket.send(data);
  };

  const isConnected = () => {
    return socket?.readyState === WebSocket.OPEN;
  };

  const getSocket = () => socket;

  return {
    connect,
    disconnect,
    send,
    isConnected,
    getSocket,
  };
}
