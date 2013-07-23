package main

import (
	"fmt"
	"code.google.com/p/go.net/websocket"
	"net/http"
	"regexp"
)

// user

type User struct {
	id		string
	websocket	*websocket.Conn
	send		chan string
}

func (u *User) reader(r *Room) {
	for {
		var message [20]byte
		n, err := u.websocket.Read(message[:])
		if err != nil  {
			break
		}

		r.broadcast <- string(message[:n])
	}

	u.websocket.Close()
}

func (u *User) writer() {
	for message := range u.send {
		err:= websocket.Message.Send(u.websocket, message)
		if err != nil {
			break
		}
	}

	u.websocket.Close()
}


// room

type Room struct {
	id		string
	users		map[*User]bool
	broadcast	chan string
	register	chan *User
	unregister	chan *User
}

func (r *Room) run() {
	for {
		select {
		case user := <-r.register:
			r.users[user] = true
			user.send <- "welcome"
		case user := <-r.unregister:
			delete(r.users, user)
		case message := <-r.broadcast:
			for current_user := range r.users {
				select {
				case current_user.send <- message:

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
	rooms		map[string]*Room
}

var h = Hub{ rooms: make(map[string]*Room) }

var (
	request_regex, _ = regexp.Compile(`/room/([0-9]+)`)
)

func DoorMan(ws *websocket.Conn) {
	user := &User{
		websocket: ws,
		send: make(chan string, 256),
	}

	path := request_regex.FindAllStringSubmatch(ws.Request().URL.Path, -1)
	room_number := path[0][1]

	room := h.rooms[room_number]
	if room == nil {
		room = &Room{
			id: room_number,
			users: make(map[*User]bool),
			broadcast: make(chan string),
			register: make(chan *User),
			unregister: make(chan *User),
		}

		h.rooms[room.id] = room
		go room.run()
	}

	fmt.Printf("current rooms: %v\n", h.rooms)

	room.register <- user
	defer func() {
		fmt.Print("defer, unregistering user!");
		room.unregister <- user
	}()

	fmt.Printf("room: %v\n", room)
	fmt.Printf("users: %v\n", room.users)
	fmt.Printf("current user: %v\n", user)

	go user.writer()
	user.reader(room)
}

func Root(c http.ResponseWriter, req *http.Request) {
	http.ServeFile(c, req, "index.html")
}

func main () {
	http.HandleFunc("/", Root)
	http.Handle("/room/", websocket.Handler(DoorMan))

	fmt.Print("Starting Server...")

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
