package main

import (
	"encoding/json"
	"github.com/garyburd/go-websocket/websocket"
	"code.google.com/p/go-sqlite/go1/sqlite3"
	"io/ioutil"
	"log"
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
	Id        string `json:"id"`
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
							User:     u,
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

				case "history":
					if m.Room != "" {
						conn, _ := sqlite3.Open("messages.db")
						defer conn.Close()

						data := make(map[string]interface{})
						messages := make([]Message, 0)

						args := sqlite3.NamedArgs{
							"$room": m.Room,
							"$limit": m.Data["limit"],
							"$offset": m.Data["offset"],
						}
						sql := "SELECT rowid, * FROM messages WHERE room = $room AND name = 'text' LIMIT $limit OFFSET $offset"
						row := make(sqlite3.RowMap)

						for s, err := conn.Query(sql, args); err == nil; err = s.Next() {
							var rowid int64
							s.Scan(&rowid, row)

							var data map[string]interface{}
							json.Unmarshal(row["data"].([]byte), &data)

							message := Message{
								Type: row["type"].(string),
								Room: row["room"].(string),
								Name: row["name"].(string),
								Data: data,
							}

							message.User.Name = row["username"].(string)
							message.User.Id = row["id"].(string)

							messages = append(messages, message)
						}

						data["messages"] = messages

						// get total
						var total int
						total_statement, _ := conn.Query("SELECT count(rowid) FROM messages WHERE room = $room AND name = 'text'", args)
						total_statement.Scan(&total);

						data["total"] = total

						new_m := Message{
							Type: TYPE_EVENT,
							Name: "history",
							Room: m.Room,
							Data: data,
						}

						log.Printf("log message: %v", new_m)
						u.send <- new_m
					}
				}
			} else {
				if room != nil && m.Type == TYPE_EVENT {
					m.User.Name = u.Name
					m.User.Id = u.Id
					room.broadcast <- m
				}
			}
		}
	}

	u.die <- true
	log.Print("user read close")
	u.websocket.Close()
}
