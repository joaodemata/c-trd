package common

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// Evento representa el payload que enviarás al frontend
type Event struct {
	TypeEvent  string  `json:"TypeEvent"`
	Data interface{}   `json:"Data"`
}

// Client envuelve la conexión de un usuario individual
type Client struct {
	Hub  *Hub
	Conn *websocket.Conn
	Send chan Event // Canal dedicado para enviar mensajes a este cliente
}

// Hub mantiene el estado de todas las conexiones activas
type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan Event
	Register   chan *Client
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Broadcast:  make(chan Event),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}


func (h *Hub) Run() {
	for {
		select {
		// Entra un nuevo cliente
		case client := <-h.Register:
			h.Clients[client] = true

		// Se desconecta un cliente
		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
			}

		// Tu API REST envía un evento para actualizar el listado
		case event := <-h.Broadcast:
			for client := range h.Clients {
				select {
				case client.Send <- event:
				default:
					// Si el canal del cliente está bloqueado, asumimos desconexión
					close(client.Send)
					delete(h.Clients, client)
				}
			}
		}
	}
}

// Configuramos el upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	//TODO: agregar environment para validar ip del front
	CheckOrigin: func(r *http.Request) bool {
		return true 
	},
}

// ServeWs maneja la petición HTTP inicial
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// 1. Upgrade de HTTP a WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error en upgrade WS: %v", err)
		return
	}

	// 2. Inicializar el cliente
	client := &Client{
		Hub:  hub,
		Conn: conn,
		Send: make(chan Event, 256), // Canal con buffer para evitar bloqueos
	}

	// 3. Registrar en el Hub
	client.Hub.Register <- client

	// 4. Iniciar los "pumps" en goroutines separadas
	// Esto permite enviar y escuchar de forma concurrente
	go client.writePump()
	go client.readPump()
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10 // Debe ser menor que pongWait
)

// readPump escucha mensajes del frontend.
// Aunque tu API solo envíe datos, esto es vital para detectar si el cliente se desconectó.
func (c *Client) readPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()
	
	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	
	// Si recibimos un pong del frontend, extendemos el tiempo límite de vida
	c.Conn.SetPongHandler(func(string) error { 
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil 
	})

	for {
		// Se queda bloqueado aquí esperando mensajes del cliente o pongs
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error de cliente WS: %v", err)
			}
			break
		}
	}
}

// writePump envía los mensajes desde el canal del Hub hacia el frontend
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		// Llega un nuevo evento desde tu API REST
		case evento, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			
			if !ok {
				// El Hub cerró el canal
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Convertir la estructura Evento a JSON y enviarla
			if err := c.Conn.WriteJSON(evento); err != nil {
				return
			}

		// Ticker para enviar Pings automáticos para mantener viva la conexión
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}