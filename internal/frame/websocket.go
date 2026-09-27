package frame

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/coder/websocket"
)

type WebSocket struct {
	conn *websocket.Conn
}

func DialWebSocket(ctx context.Context, client *http.Client, url, origin string) (*WebSocket, error) {
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient: client,
		HTTPHeader: http.Header{"Origin": {origin}},
	})
	if err != nil {
		return nil, fmt.Errorf("frame: dial %s: %w", url, err)
	}
	conn.SetReadLimit(MaxSize + lengthSize)
	return &WebSocket{conn: conn}, nil
}

func (w *WebSocket) ReadMessage(ctx context.Context) ([]byte, error) {
	for {
		kind, data, err := w.conn.Read(ctx)
		if err != nil {
			return nil, err
		}
		if kind == websocket.MessageBinary {
			return data, nil
		}
	}
}

func (w *WebSocket) WriteMessage(ctx context.Context, message []byte) error {
	return w.conn.Write(ctx, websocket.MessageBinary, message)
}

func (w *WebSocket) Close() error {
	if err := w.conn.Close(websocket.StatusNormalClosure, ""); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}
