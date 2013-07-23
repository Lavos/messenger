package main

import (
	"fmt"
	"code.google.com/p/go.net/websocket"
	"io"
	"net/http"
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
			fmt.Print("here!")
			fmt.Printf("%v", user)
			r.users[user] = true
		case user := <-r.unregister:
			delete(r.users, user)
		case message := <-r.broadcast:
			fmt.Print("got message!")
			fmt.Printf("%v", message)
			fmt.Printf("%v", r)

			for current_user := range r.users {
				fmt.Printf("%v", current_user)

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

func EchoServer(ws *websocket.Conn) {
	io.Copy(ws, ws)
}

func DoorMan(ws *websocket.Conn) {
	user := &User{
		websocket: ws,
		send: make(chan string, 256),
	}

	room := h.rooms["room1"]
	if room == nil {
		room = &Room{
			id: "room1",
			users: make(map[*User]bool),
			broadcast: make(chan string),
			register: make(chan *User),
			unregister: make(chan *User),
		}

		h.rooms[room.id] = room
		go room.run()
	}

	room.register <- user
	defer func() {
		fmt.Print("defer!");
		room.unregister <- user
	}()

	fmt.Printf("%v", room)
	fmt.Printf("%v", user)

	go user.writer()
	user.reader(room)
}

func Root(c http.ResponseWriter, req *http.Request) {
	http.ServeFile(c, req, "index.html")
}

func main () {
	fmt.Printf("%v", h)
	http.HandleFunc("/", Root)
	http.Handle("/echo", websocket.Handler(EchoServer))
	http.Handle("/room1", websocket.Handler(DoorMan))

	fmt.Print("Starting Server...")

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
