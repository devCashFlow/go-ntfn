package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	_ "github.com/go-sql-driver/mysql"
)

type Client struct {
	conn    *websocket.Conn
	session map[int]bool
}

type Notification struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

var (
	clients      = make(map[*Client]bool)
	mu           sync.Mutex
	notification = make(chan Notification)
	db           *sql.DB
)

func main() {
	var err error
	db, err = sql.Open("mysql", "root:123@tcp(127.0.0.1:3306)/notification_db")
	if err != nil {
		fmt.Println("Error connecting to database:", err)
		return
	}
	defer db.Close()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "site/index.html")
	})

	http.HandleFunc("/ws", handleConnections)

	go handleNotifications()
	go scheduler()

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func scheduler() {
	for {
		rows, err := db.Query("SELECT id, message FROM notifications WHERE ack = false")
		if err != nil {
			fmt.Println("Error fetching notifications:", err)
			continue
		}
		for rows.Next() {
			var ntfn Notification
			if err := rows.Scan(&ntfn.ID, &ntfn.Message); err != nil {
				fmt.Println("Error scanning row:", err)
				continue
			}
			notification <- ntfn
		}
		rows.Close()

		time.Sleep(10 * time.Second)
	}
}

func handleNotifications() {
	for {
		notif := <-notification

		mu.Lock()
		for client := range clients {
			if !client.session[notif.ID] {
				err := wsjson.Write(context.Background(), client.conn, notif)
				if err != nil {
					fmt.Println("Error sending notification:", err)
					client.conn.Close(websocket.StatusInternalError, "the sky is falling")
					delete(clients, client)
				} else {
					client.session[notif.ID] = true
				}
			}
		}
		mu.Unlock()
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	acceptOptions := &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		OriginPatterns:     []string{"*"},
	}

	conn, err := websocket.Accept(w, r, acceptOptions)
	if err != nil {
		fmt.Println("Error accepting connection:", err)
		return
	}

	client := &Client{conn: conn, session: make(map[int]bool)}

	mu.Lock()
	clients[client] = true
	mu.Unlock()

	// This code reads from the WebSocket connection to keep it open.
	// You may want to handle incoming messages here.
	var v Notification
	for {
		err := wsjson.Read(r.Context(), conn, &v)
		if err != nil {
			delete(clients, client)
			return
		}
		// handle ack
		go acknowledgeNotification(v.ID)
	}
}

func acknowledgeNotification(id int) {
	_, err := db.Exec("UPDATE notifications SET ack = true WHERE id = ?", id)
	if err != nil {
		fmt.Println("Error acknowledging notification:", err)
		return
	}

	// Send acknowledgment message
	mu.Lock()
	for client := range clients {
		err := wsjson.Write(context.Background(), client.conn, Notification{ID: id, Message: "ACK"})
		if err != nil {
			fmt.Println("Error sending acknowledgment:", err)
		}
	}
	mu.Unlock()
}
