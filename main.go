package main

import (
	"log"
	"net/http"
	"strings"
	"time" // <-- この行を追加

	"github.com/gorilla/websocket"
)

//var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan Message)
var upgrader = websocket.Upgrader{}
var clients = make(map[*websocket.Conn]bool)        // connected clients
var participants = make(map[*websocket.Conn]string) // client connection -> username

type Message struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}
type TimedMessage struct {
	Msg       Message
	Timestamp time.Time
}

var messageLog []TimedMessage

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "public/join.html")
	})

	http.HandleFunc("/chat.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "public/chat.html")
	})

	http.HandleFunc("/ws", handleConnections)

	go handleMessages()

	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	clients[ws] = true
	for _, logEntry := range messageLog {
		err := ws.WriteJSON(logEntry.Msg)
		if err != nil {
			log.Printf("Error sending message: %v", err)
			break
		}
	}
	for {
		var msg Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			delete(clients, ws)
			delete(participants, ws)
			broadcastParticipants() // Broadcast the updated participant list
			break
		}

		// If user is not already in the participants list, add and broadcast
		if _, ok := participants[ws]; !ok {
			participants[ws] = msg.Name
			broadcastParticipants() // Broadcast the new participant
		}

		// Add the received message to the log
		addMessageToLog(msg)

		// Broadcast the received message to all clients
		broadcast <- msg
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
	}
}
func addMessageToLog(msg Message) {
	now := time.Now()
	messageLog = append(messageLog, TimedMessage{Msg: msg, Timestamp: now})

	// Remove messages that are older than 1 hour
	cutoff := now.Add(-1 * time.Hour)
	for i, m := range messageLog {
		if m.Timestamp.After(cutoff) {
			// Slice the array to only keep messages after the cutoff
			messageLog = messageLog[i:]
			break
		}
	}
}

func broadcastParticipants() {
	var list []string
	for _, name := range participants {
		list = append(list, name)
	}

	message := Message{
		Name:    "SERVER",
		Content: strings.Join(list, ","),
	}

	for client := range clients {
		err := client.WriteJSON(message)
		if err != nil {
			log.Printf("WebSocket error: %v", err)
			client.Close()
			delete(clients, client)
			delete(participants, client)
		}
	}
}
