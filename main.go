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

func (u *User) reader() {
	for {
		var message [20]byte
		n, err := u.websocket.Read(message[:])
		if err != nil  {
			break
		}

		h.broadcast <- string(message[:n])
	}

	u.websocket.Close()
}

func (u *User) writer() {
	for message := range u.send {
		err:= websocket.Message.send(u.websocket, message)
		if err != nil {
			break
		}
	}

	u.websocket.Close()
}


// room

type Room struct {
	users		map[string]*User
	broadcast	chan string
	join		chan *User
	part		chan *User
}

func (r *Room) run() {
	for {
		select {
		case u:= <-r.join:
			r.users[u.id] = u

		case u:= <-r.part:
			delete(r.users, u.id)

		case u:= <-r.broadcast:
			for current_user:= range r.users {
				select {
				case current_user.send <- u:

				default:
					delete(r.users, current_user)
					close(current_user.send)
					go c.ws.Close()
				}
			}
		}
	}
}

// hub

type Hub struct {
	rooms		map[string]*Room
	register	chan *Room
	unregister	chan *Room
}









func EchoServer(ws *websocket.Conn) {
	io.Copy(ws, ws)
}

func Root(c http.ResponseWriter, req *http.Request) {
	http.ServeFile(c, req, "index.html")
}

func main () {
	http.HandleFunc("/", Root)
	http.Handle("/echo", websocket.Handler(EchoServer))
	fmt.Print("Starting Server...")

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
