package main

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"net/http"
	"regexp"
	// "encoding/json"
)

// user

const (
	TYPE_STATUS = "status"
	TYPE_MESSAGE = "message"
)

type Message struct {
	MessageType string
	Status Status
	Text string
}

type Status struct {
	Users int
}

type User struct {
	id        string
	websocket *websocket.Conn
	send      chan Message
}

func (u *User) reader(r *Room) {
	for {
		var content string
		err := websocket.Message.Receive(u.websocket, &content)

		if err != nil {
			return
		}

		m := Message{
			MessageType: TYPE_MESSAGE,
			Text: content,
		}

		r.broadcast <- m
	}

	u.websocket.Close()
}

func (u *User) writer() {
	for message := range u.send {
		err := websocket.JSON.Send(u.websocket, message)
		if err != nil {
			break
		}
	}

	u.websocket.Close()
}

// room

type Room struct {
	id         string
	users      map[*User]bool
	broadcast  chan Message
	register   chan *User
	unregister chan *User
}

func (r *Room) run(h *Hub) {
	for {
		select {
		case user := <-r.register:
			r.users[user] = true

			m := Message{
				MessageType: TYPE_STATUS,
				Status: Status{
					Users: len(r.users),
				},
			}

			user.send <- m
			fmt.Printf("current users: %v\n", r.users)
		case user := <-r.unregister:
			fmt.Print("Unregister!\n")
			delete(r.users, user)
			close(user.send)
			fmt.Printf("current users: %v\n", r.users)

			if len(r.users) == 0 {
				fmt.Print("I'm now empty, unregistering.\n")
				h.unregister <- r
			}
		case message := <-r.broadcast:
			for current_user := range r.users {
				select {
				case current_user.send <- message:

				// if stuck or dead
				default:
					delete(r.users, current_user)
					close(current_user.send)
					go current_user.websocket.Close()
				}
			}
		}
	}
}

// hub

type Hub struct {
	rooms	   map[string]*Room
	unregister chan *Room
	join	   chan *RoomRequest
	booking    chan *Room
}

func (h *Hub) Run() {
	for {
		select {
		case room := <-h.unregister:
			delete(h.rooms, room.id)
			fmt.Printf("current rooms: %v\n", h.rooms)

		case request := <-h.join:
			room := h.rooms[request.name]
			if room == nil {
				room = &Room{
					id:         request.name,
					users:      make(map[*User]bool),
					broadcast:  make(chan Message),
					register:   make(chan *User),
					unregister: make(chan *User),
				}

				h.rooms[room.id] = room
				go room.run(h)
			}

			fmt.Printf("current rooms: %v\n", h.rooms)
			request.booking <-room
		}
	}
}


var h = Hub{
	rooms: make(map[string]*Room),
	unregister: make(chan *Room),
	join: make(chan *RoomRequest),
	booking: make(chan *Room),
}

type RoomRequest struct {
	name	string
	booking	chan *Room
}

var (
	request_regex, _ = regexp.Compile(`/room/([0-9]+)`)
)

func DoorMan(ws *websocket.Conn) {
	path := request_regex.FindAllStringSubmatch(ws.Request().URL.Path, -1)

	if path == nil {
		return
	}

	room_number := path[0][1]

	user := &User{
		websocket: ws,
		send:      make(chan Message),
	}

	request := &RoomRequest{
		name: room_number,
		booking: make(chan *Room),
	}

	h.join <- request
	room := <-request.booking

	room.register <- user
	defer func() {
		fmt.Print("defer, unregistering user!\n")
		room.unregister <- user
	}()

	go user.writer()
	user.reader(room) // blocks until websocket is closed
}

func Root(c http.ResponseWriter, req *http.Request) {
	http.ServeFile(c, req, "index.html")
}

func main() {
	go h.Run()
	http.HandleFunc("/", Root)
	http.Handle("/room/", websocket.Handler(DoorMan))

	fmt.Print("Started Server.\n")

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}

}
