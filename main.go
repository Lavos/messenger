package main

import (
	"code.google.com/p/go.net/websocket"
	"fmt"
	"net/http"
	"regexp"
)

// user

type User struct {
	id        string
	websocket *websocket.Conn
	send      chan string
}

func (u *User) reader(r *Room) {
	for {
		var message [20]byte
		n, err := u.websocket.Read(message[:])
		if err != nil {
			break
		}

		r.broadcast <- string(message[:n])
	}

	u.websocket.Close()
}

func (u *User) writer() {
	for message := range u.send {
		err := websocket.Message.Send(u.websocket, message)
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
	broadcast  chan string
	register   chan *User
	unregister chan *User
}

func (r *Room) run(h *Hub) {
	for {
		select {
		case user := <-r.register:
			r.users[user] = true
			user.send <- "welcome"
		case user := <-r.unregister:
			fmt.Print("Unregister!\n")

			delete(r.users, user)

			if len(r.users) == 0 {
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
	join	   chan string
	booking    chan *Room
}

func (h *Hub) Run() {
	for {
		select {
		case room := <-h.unregister:
			delete(h.rooms, room.id)
			fmt.Printf("%v\n", h.rooms)

		case name := <-h.join:
			room := h.rooms[name]
			if room == nil {
				room = &Room{
					id:         name,
					users:      make(map[*User]bool),
					broadcast:  make(chan string),
					register:   make(chan *User),
					unregister: make(chan *User),
				}

				h.rooms[room.id] = room
				go room.run(h)
			}

			fmt.Printf("%v\n", h.rooms)
			h.booking <-room
		}
	}
}


var h = Hub{
	rooms: make(map[string]*Room),
	unregister: make(chan *Room),
	join: make(chan string),
	booking: make(chan *Room),
}

var (
	request_regex, _ = regexp.Compile(`/room/([0-9]+)`)
)

func DoorMan(ws *websocket.Conn) {
	user := &User{
		websocket: ws,
		send:      make(chan string, 256),
	}

	path := request_regex.FindAllStringSubmatch(ws.Request().URL.Path, -1)
	room_number := path[0][1]

	h.join <- room_number
	room := <-h.booking

	room.register <- user
	defer func() {
		fmt.Print("defer, unregistering user!\n")
		room.unregister <- user
	}()

	go user.writer()
	user.reader(room)
}

func Root(c http.ResponseWriter, req *http.Request) {
	http.ServeFile(c, req, "index.html")
}

func main() {
	go h.Run()
	http.HandleFunc("/", Root)
	http.Handle("/room/", websocket.Handler(DoorMan))

	fmt.Print("Starting Server...\n")

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}

}
