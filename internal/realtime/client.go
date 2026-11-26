package realtime

import (
	"context"
	"time"

	"github.com/coder/websocket"
)

// pingInterval keeps the connection warm through nginx and Cloudflare idle
// timeouts. CloseRead sends pongs automatically; we only need to send pings.
const pingInterval = 30 * time.Second

// Client is one open WebSocket connection belonging to a signed-in user.
type Client struct {
	userID int64
	conn   *websocket.Conn

	// send buffers outgoing frames. Only the write pump reads it, so there is a
	// single writer per connection as the library requires.
	send chan []byte
}

// NewClient wires a freshly accepted connection to a user.
func NewClient(userID int64, conn *websocket.Conn) *Client {
	return &Client{
		userID: userID,
		conn:   conn,
		send:   make(chan []byte, sendBuffer),
	}
}

// Serve registers the client and runs the write pump until the connection
// closes, then unregisters it. It blocks, so call it from the HTTP handler.
func (h *Hub) Serve(ctx context.Context, c *Client) {
	h.add(c)
	defer h.remove(c)
	defer c.conn.CloseNow()

	// CloseRead runs the required read loop (handling pings, pongs and close
	// frames) and returns a context that is cancelled when the peer goes away.
	readCtx := c.conn.CloseRead(ctx)

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-readCtx.Done():
			return
		case msg := <-c.send:
			writeCtx, cancel := context.WithTimeout(readCtx, 10*time.Second)
			err := c.conn.Write(writeCtx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				return
			}
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(readCtx, 10*time.Second)
			err := c.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		}
	}
}
