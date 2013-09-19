package main

import (
	"log"
	"net/http"
	"runtime"
	"github.com/garyburd/go-websocket/websocket"
)

const (
	TYPE_COMMAND = "command"
	TYPE_EVENT   = "event"
)

type Message struct {
	Type string                 `json:"type,omitempty"`
	Room string                 `json:"room,omitempty"`
	Name string                 `json:"name,omitempty"`
	Data map[string]interface{} `json:"data,omitempty"`
	User struct {
		Name      string `json:"name"`
		Id        string `json:"id"`
	} `json:"user,omitempty"`
}

type RoomRequest struct {
	User     *User
	RoomName string
}

var h = Hub{
	rooms:      make(map[string]*Room),
	unregister: make(chan *Room),
	join:       make(chan *RoomRequest),
	updates:    make(chan Message),
}

func DoorMan(w http.ResponseWriter, r *http.Request) {
	ws, err := websocket.Upgrade(w, r.Header, nil, 1024, 1024)

	if err != nil {
		log.Print("Could not upgrade connection.")
		return
	}

	r.ParseForm()
	user_name := r.Form.Get("user_name")
	user_id := r.Form.Get("user_id")

	user := &User{
		websocket: ws,
		send:      make(chan Message, 24),
		die:       make(chan bool),
		join:      make(chan *Room),
		Name:      user_name,
		Id:        user_id,
		rooms:     make(map[string]*Room),
	}

	user.Run() // blocks until websocket is closed
	log.Print("DoorMan close")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	go h.Run()
	http.HandleFunc("/room", DoorMan)

	log.Print("Started Server.")

	/* go func() {
		c := time.Tick(5 * time.Second)
		for now := range c {
			log.Printf("- %v - go routines: %v", now, runtime.NumGoroutine())
		}
	}() */

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
