package main

import (
	"log"
	"github.com/Lavos/bucket"
)

type Hub struct {
	rooms      map[string]*Room
	unregister chan *Room
	join       chan *RoomRequest
	updates    chan Message
}

func (h *Hub) Run() {
	global := h.CreateRoom("global", false)

	for {
		select {
		case room := <-h.unregister:
			delete(h.rooms, room.id)

		case request := <-h.join:
			room := h.rooms[request.RoomName]
			if room == nil {
				room = h.CreateRoom(request.RoomName, true)
			}

			request.User.join <- room

			if request.RoomName == "global" {
				go h.GetCurrentStatus(request.User.send)
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
		history:    make(chan chan Message),
		log:	    make(chan chan Message),
		chatlog:    bucket.NewBucket(50),
		autoclose:  autoclose,
	}

	log.Printf("[hub] created room: %v", room.id)
	log.Printf("[hub] %v", room)

	h.rooms[room.id] = room
	go room.Run(h)
	return room
}
