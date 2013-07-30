package main

import (
	"code.google.com/p/go.net/websocket"
	"log"
	"net/http"
	"runtime"
	"time"
)

const (
	TYPE_STATUS = "status"
	TYPE_TEXT   = "text"
)

type Message struct {
	MessageType string   `json:"messagetype"`
	UserList    []string `json:"user_list,omitempty"`
	RoomName    string   `json:"room_name,omitempty"`
	Text        string   `json:"text,omitempty"`
	User        string   `json:"user,omitempty"`
}

// User

type User struct {
	id        string
	websocket *websocket.Conn
	send      chan Message
	read      chan Message
	join      chan *Room
	die       chan bool
	name      string
	room_name string
	room      *Room
}

func (u *User) run() {
	go u.reader()

	defer log.Print("user run close")
	defer u.websocket.Close()

	for {
		select {

		case room := <-u.join:
			u.room = room
			u.room.register <- u

		case <-u.die:
			log.Print("user die.")
			u.room.unregister <- u
			return

		case message := <-u.read:
			u.room.broadcast <- message

		case message := <-u.send:
			log.Printf("[%v] got message: %v", u.name, message)

			err := websocket.JSON.Send(u.websocket, message)
			if err != nil {
				return
			}
		}
	}
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
			Text:        content,
			User:        u.name,
		}

		u.read <- m
	}

	u.die <- true
	u.websocket.Close()
	log.Print("user read close")
}

// room

type Room struct {
	id          string
	users       map[*User]bool
	broadcast   chan Message
	status      chan chan Message
	register    chan *User
	unregister  chan *User
	statusTimer *time.Timer
	autoclose   bool
}

func (r *Room) run(h *Hub) {
	for {
		select {
		case user := <-r.register:
			r.users[user] = true
			r.SendStatus()
		case user := <-r.unregister:
			delete(r.users, user)
			r.SendStatus()

			if r.autoclose && len(r.users) == 0 {
				log.Printf("[%v] I'm now empty, unregistering.\n", r.id)
				h.unregister <- r
			}
		case message := <-r.broadcast:
			r.SendToUsers(message)
		case returnchan := <-r.status:
			m := Message{
				MessageType: TYPE_STATUS,
				UserList:    r.GetUserList(),
				RoomName:    r.id,
			}

			log.Printf("requested status: %v", m)
			returnchan <- m
		}
	}

	log.Printf("Room %s close.", r.id)
}

func (r *Room) GetUserList() []string {
	list := make([]string, 0, len(r.users))

	for user, _ := range r.users {
		list = append(list, user.name)
	}

	return list
}

func (r *Room) SendStatus() {
	if r.statusTimer != nil {
		r.statusTimer.Stop()
		r.statusTimer = nil
	}

	r.statusTimer = time.AfterFunc(1*time.Second, func() {
		list := r.GetUserList()

		m := Message{
			MessageType: TYPE_STATUS,
			UserList:    list,
			RoomName:    r.id,
		}

		log.Printf("[%v] current users: %v\n", r.id, len(r.users))
		r.SendToUsers(m)

		h.updates <- m
	})
}

func (r *Room) SendToUsers(m Message) {
	for current_user := range r.users {
		current_user.send <- m
	}
}

// hub

type Hub struct {
	rooms      map[string]*Room
	unregister chan *Room
	join       chan *User
	updates    chan Message
}

func (h *Hub) Run() {
	global := h.CreateRoom("global", false)

	for {
		select {
		case room := <-h.unregister:
			delete(h.rooms, room.id)

		case user := <-h.join:
			room := h.rooms[user.room_name]
			if room == nil {
				room = h.CreateRoom(user.room_name, true)
			}

			user.join <- room
			if user.room_name == "global" {
				go h.GetCurrentStatus(user.send)
			}

		case message := <-h.updates:
			global.broadcast <- message
		}
	}
}

func (h *Hub) GetCurrentStatus(returnchan chan Message) {
	for _, reg_room := range h.rooms {
		reg_room.status <- returnchan
	}
}

func (h *Hub) CreateRoom(name string, autoclose bool) *Room {
	room := &Room{
		id:         name,
		users:      make(map[*User]bool),
		broadcast:  make(chan Message),
		register:   make(chan *User),
		unregister: make(chan *User),
		status:     make(chan chan Message),
		autoclose:  autoclose,
	}

	log.Printf("[hub] created room: %v", room.id)

	h.rooms[room.id] = room
	go room.run(h)
	return room
}

var h = Hub{
	rooms:      make(map[string]*Room),
	unregister: make(chan *Room),
	join:       make(chan *User),
	updates:    make(chan Message),
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
		die:       make(chan bool),
		join:      make(chan *Room),
		name:      user_name,
		room_name: room_name,
	}

	h.join <- user

	user.run() // blocks until websocket is closed

	log.Print("DoorMan close")
}

func Root(c http.ResponseWriter, req *http.Request) {
	http.ServeFile(c, req, "index.html")
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	go h.Run()
	http.HandleFunc("/", Root)
	http.Handle("/room", websocket.Handler(DoorMan))

	log.Print("Started Server.")

	if err := http.ListenAndServe(":12345", nil); err != nil {
		panic("ListenAndServe: " + err.Error())
	}
}
