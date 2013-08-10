package main

import (
	"log"
	"io/ioutil"
	"github.com/garyburd/go-websocket/websocket"
	"encoding/json"
	"time"
)

const (
	// Time allowed to write a message to the client.
	writeWait = 10 * time.Second

	// Time allowed to read the next message from the client.
	readWait = 60 * time.Second

	// Send pings to client with this period. Must be less than readWait.
	pingPeriod = (readWait * 9) / 10

	// Maximum message size allowed from client.
	maxMessageSize = 512
)

type User struct {
	websocket *websocket.Conn
	send      chan Message
	join      chan *Room
	die       chan bool
	Name      string `json:"name"`
	Id	  string `json:"id"`
	rooms     map[string]*Room
}

func (u *User) Run() {
	go u.Reader()

	ticker := time.NewTicker(15 * time.Second)
	defer func() {
		ticker.Stop()
		log.Print("user run close")
		u.websocket.Close()
	}()

	for {
		select {

		case room := <-u.join:
			u.rooms[room.id] = room
			room.register <- u

		case <-u.die:
			log.Print("user die signal.")

			for _, room := range u.rooms {
				log.Printf("[%v] attempting to unregister from: [%v]", u.Name, room.id)
				room.unregister <- u
				log.Printf("[%v] unregister success from: [%v]", u.Name, room.id)
			}

			log.Print("user unregister from die.")
			return

		case message := <-u.send:
			log.Printf("[%v] sending to client message: %v", u.Name, message)
			b, err := json.Marshal(message)

			if err != nil {
				return
			}

			werr := u.Write(websocket.OpText, b)

			if werr != nil {
				return
			}

		case <-ticker.C:
			if err := u.Write(websocket.OpPing, []byte{}); err != nil {
				log.Printf("[%v] ping failed, closing.", u.Name)
				return
			}
		}
	}
}

func (u *User) Write(opCode int, payload []byte) error {
	u.websocket.SetWriteDeadline(time.Now().Add(writeWait))
	return u.websocket.WriteMessage(opCode, payload)
}

func (u *User) Reader() {
	for {
		op, r, err := u.websocket.NextReader()
		if err != nil {
			log.Printf("[%v] got a misformed JSON message from browser, or websocket close.", u.Name)
			break
		}

		switch op {
		case websocket.OpPong:
			u.websocket.SetReadDeadline(time.Now().Add(readWait))
		case websocket.OpText:
			blob, err := ioutil.ReadAll(r)
			if err != nil {
				break
			}

			var m Message
			merr := json.Unmarshal(blob, &m)

			if merr != nil {
				break
			}

			room := u.rooms[m.Room]

			if m.Type == TYPE_COMMAND {
				switch m.Name {
				case "join":
					if room == nil {
						r := &RoomRequest{
							User: u,
							RoomName: m.Room,
						}

						h.join <- r
					}

				case "part":
					if room != nil {
						log.Printf("[%v] attempting to unregister from: [%v]", u.Name, room.id)
						u.rooms[m.Room].unregister <- u
						log.Printf("[%v] unregister success from: [%v]", u.Name, room.id)
						delete(u.rooms, m.Room)
					}
				}
			} else {
				if room != nil && m.Type == TYPE_EVENT  {
					m.User = u
					room.broadcast <- m
				}
			}
		}
	}

	u.die <- true
	log.Print("user read close")
	u.websocket.Close()
}
