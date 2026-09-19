package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"pollapp/internal/hub"
)

type WSHandler struct {
	Hub *hub.Hub
}

var upgrader = websocket.Upgrader{
	// Poll results are public data shared via a link, so any origin may connect.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// PollSocket upgrades to a websocket and subscribes this connection to
// live results for one poll, via the Hub -> Redis pub/sub chain.
func (h *WSHandler) PollSocket(c *gin.Context) {
	pollID := c.Param("id")

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}

	h.Hub.Register(pollID, conn)
	defer h.Hub.Unregister(pollID, conn)

	// We don't expect messages from the client on this socket; just block
	// on reads so we notice when the connection closes.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}
