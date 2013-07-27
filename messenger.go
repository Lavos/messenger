package main

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"net/http"
)

// user

const (
	TYPE_STATUS = "status"
	TYPE_TEXT = "text"
)

type Message struct {
	MessageType string `json:"messagetype"`
	Status Status `json:"status,omitempty"`
	Text string `json:"text,omitempty"`
	User string `json:"user,omitempty"`
}

type Status struct {
	Users int `json:"users,omitempty"`
}

type User struct {
	id        string
	websocket *websocket.Conn
	send      chan Message
	name	  string
}

func (u *User) reader(r *Room) {
	for {
		var content string
		err := websocket.Message.Receive(u.websocket, &content)

		if err != nil {
			continue
		}

		m := Message{
			MessageType: TYPE_TEXT,
			Text: content,
			User: u.name,
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

			/*m := Message{
				MessageType: TYPE_STATUS,
				Status: Status{
					Users: len(r.users),
				},
			}

			go func() { r.broadcast <- m }()*/
			 fmt.Printf("current users: %v\n", len(r.users))
		case user := <-r.unregister:
			// fmt.Print("Unregister!\n")
			delete(r.users, user)
			close(user.send)
			fmt.Printf("current users: %v\n", len(r.users))

			if len(r.users) == 0 {
				// fmt.Print("I'm now empty, unregistering.\n")
				h.unregister <- r
			} else {

				/*m := Message{
					MessageType: TYPE_STATUS,
					Status: Status{
						Users: len(r.users),
					},
				}

				go func() { r.broadcast <- m }()*/
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
}

func (h *Hub) Run() {
	for {
		select {
		case room := <-h.unregister:
			delete(h.rooms, room.id)
			// fmt.Printf("current rooms: %v\n", h.rooms)

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

			// fmt.Printf("current rooms: %v\n", h.rooms)
			request.booking <-room
		}
	}
}


var h = Hub{
	rooms: make(map[string]*Room),
	unregister: make(chan *Room),
	join: make(chan *RoomRequest),
}

type RoomRequest struct {
	name	string
	booking	chan *Room
}

func DoorMan(ws *websocket.Conn) {
	ws.Request().ParseForm()
	room_name := ws.Request().Form.Get("name")
	user_name := ws.Request().Form.Get("user_name")

	if len(room_name) == 0 {
		return
	}

	user := &User{
		websocket: ws,
		send:      make(chan Message),
		name: user_name,
	}

	request := &RoomRequest{
		name: room_name,
		booking: make(chan *Room),
	}

	h.join <- request
	room := <-request.booking

	room.register <- user
	defer func() {
		// fmt.Print("defer, unregistering user!\n")
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
	http.Handle("/room", websocket.Handler(DoorMan))

	fmt.Print("Started Server.\n")

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}

}
