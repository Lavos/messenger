package main

import (
	"log"
	"time"
	"github.com/Lavos/bucket"
	"code.google.com/p/go-sqlite/go1/sqlite3"
	"encoding/json"
)

type Room struct {
	id          string
	users       map[*User]bool
	broadcast   chan Message
	status      chan chan Message
	register    chan *User
	unregister  chan *User
	history     chan chan Message
	log	    chan chan Message
	chatlog	    *bucket.Bucket
	statusTimer *time.Timer
	autoclose   bool
}

func (r *Room) Run(h *Hub) {
	defer log.Printf("[%v] Room run close.", r.id)

	conn, _ := sqlite3.Open("messages.db")
	defer conn.Close()

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
				return
			}
		case message := <-r.broadcast:
			r.SendToUsers(message)

			if message.Type == TYPE_EVENT {
				r.chatlog.Bump(message)

				b, _ := json.Marshal(message.Data)

				args := sqlite3.NamedArgs{
					"$type": message.Type,
					"$room": message.Room,
					"$name": message.Name,
					"$data": b,
					"$username": message.User.Name,
					"$id": message.User.Id,
				}

				log.Printf("args: %v", args)

				insert_err := conn.Exec("INSERT INTO messages (type, room, name, data, username, id) VALUES ($type, $room, $name, $data, $username, $id);", args)
				if insert_err != nil {
					log.Printf("insert_err: %v", insert_err)
				}
				conn.Commit()
			}
		case returnchan := <-r.status:
			m := r.BuildStatusMessage()
			log.Printf("requested status: %v", m)
			returnchan <- m
		case returnchan := <-r.log:
			m := Message{
				Type: TYPE_EVENT,
				Name: "log",
				Room: r.id,
				Data: r.chatlog.Get(),
			}

			returnchan <- m
		case returnchan := <-r.history:
			for _, m := range r.chatlog.Get() {
				returnchan <- m.(Message)
			}
		}
	}
}

func (r *Room) BuildStatusMessage() Message {
	data := make(map[string]interface{})
	data["user_list"] = r.GetUserList()

	return Message{
		Type: TYPE_EVENT,
		Name: "status",
		Room: r.id,
		Data: data,
	}
}

func (r *Room) GetUserList() []string {
	list := make([]string, 0, len(r.users))

	for user, _ := range r.users {
		list = append(list, user.Name)
	}

	return list
}

func (r *Room) SendStatus() {
	if r.statusTimer != nil {
		r.statusTimer.Stop()
		r.statusTimer = nil
	}

	r.statusTimer = time.AfterFunc(1*time.Second, func() {
		m := r.BuildStatusMessage()
		log.Printf("[%v] current users: %v\n", r.id, len(r.users))
		r.SendToUsers(m)

		h.updates <- m
	})
}

func (r *Room) SendToUsers(m Message) {
	for current_user := range r.users {
		select {
		case current_user.send <- m:
		default:
		}
	}
}
