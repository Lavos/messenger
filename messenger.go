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
	read	  chan Message
	join	  chan *Room
	die	  chan bool
	name	  string
	room	  *Room
}

func (u *User) run() {
	go u.reader()

	for {
		select {

		case room := <-u.join:
			u.room = room
			u.room.register <- u

		case <-u.die:
			fmt.Print("DIE!\n")
			u.room.unregister <- u
			break

		case message := <-u.read:
			u.room.broadcast <- message

		case message := <-u.send:
			err := websocket.JSON.Send(u.websocket, message)
			if err != nil {
				break
			}
		}
	}

	u.websocket.Close()
}

func (u *User) reader() {
	for {
		var content string
		err := websocket.Message.Receive(u.websocket, &content)

		if err != nil {
			break
		}

		m := Message{
			MessageType: TYPE_TEXT,
			Text: content,
			User: u.name,
		}

		u.read <- m
	}

	u.die <- true
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

			r.SendToUsers(m)
			fmt.Printf("current users: %v\n", len(r.users))
		case user := <-r.unregister:
			// fmt.Print("Unregister!\n")
			delete(r.users, user)
			fmt.Printf("current users: %v\n", len(r.users))

			if len(r.users) == 0 {
				fmt.Print("I'm now empty, unregistering.\n")
				h.unregister <- r
			} else {
				m := Message{
					MessageType: TYPE_STATUS,
					Status: Status{
						Users: len(r.users),
					},
				}

				r.SendToUsers(m)
			}
		case message := <-r.broadcast:
			r.SendToUsers(message)
		}
	}
}

func (r *Room) SendToUsers(m Message) {
	for current_user := range r.users {
		select {
		case current_user.send <- m:
		default:
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
			request.booking <- room
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
		read:      make(chan Message),
		die:	   make(chan bool),
		join:      make(chan *Room),
		name: user_name,
	}

	request := &RoomRequest{
		name: room_name,
		booking: user.join,
	}

	h.join <- request

	user.run() // blocks until websocket is closed
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
