package server

import (
	"encoding/json"
	"errors"
	"net"
	"sync"

	"github.com/google/uuid"
)

const RoomCapacity = 10

type Room struct {
	Name       string
	ID         uuid.UUID
	Capacity   int
	NumPlayers int
	Players    []Player
	Lock       *sync.Mutex
}

type Player struct {
	ID      uuid.UUID
	Addr    string
	UDPPort int
	TCPPort int
}

type RoomState struct {
	Rooms    map[uuid.UUID]Room
	Players  map[uuid.UUID]Player
	Lock     *sync.Mutex
	ErrChan  chan error
	RecvChan chan []byte
}

func (p *Player) UDPAddr() *net.UDPAddr {
	return &net.UDPAddr{IP: []byte(p.Addr), Port: p.UDPPort}
}

func (p *Player) TCPAddr() *net.TCPAddr {
	return &net.TCPAddr{IP: []byte(p.Addr), Port: p.TCPPort}
}

func (p *Player) SendUDP(pidFrom *uuid.UUID, msg string) error {
	conn, err := net.DialUDP("udp", nil, p.UDPAddr())
	playerId := pidFrom.String()
	msgMap := map[string]string{playerId: msg}
	msgJson, err := json.Marshal(msgMap)
	if err != nil {
		return err
	}
	_, err = conn.Write(msgJson)
	if err != nil {
		return err
	}

	return nil
}

func (p *Player) SendTCP(status string, data string, conn *net.TCPConn) error {
	switch status {
	case "ok", "fail":
	default:
		return errors.New("wrong status " + status)
	}
	msg := map[string]string{"status": status, "data": data}
	msgJson, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = conn.Write(msgJson)
	if err != nil {
		return err
	}

	return nil
}

func (r Room) IsEmpty() bool {
	if r.NumPlayers == 0 {
		return true
	}

	return false
}

func (r *Room) IsFull() bool {
	if r.Capacity >= RoomCapacity {
		return true
	}

	return false
}

func (r *Room) InRoom(pid *uuid.UUID) bool {
	for _, player := range r.Players {
		if player.ID == *pid {
			return true
		}
	}

	return false
}

func (r *Room) Join(player *Player) error {
	if r.IsFull() {
		return errors.New("room is full")
	}

	r.Lock.Lock()
	r.NumPlayers = r.NumPlayers + 1
	r.Players = append(r.Players, *player)
	r.Lock.Unlock()

	return nil
}

func (r *Room) Leave(player Player) (pid *uuid.UUID) {
	players := make([]Player, 0)
	for _, pl := range r.Players {
		if pl.ID != player.ID {
			players = append(players, pl)
		} else {
			pid = &pl.ID
		}
	}
	r.Lock.Lock()
	r.Players = players
	r.NumPlayers = r.NumPlayers - 1
	r.Lock.Unlock()

	return pid
}

func NewRoomState(recvChan chan []byte, errChan chan error) (*RoomState, error) {
	return &RoomState{
		Rooms:    make(map[uuid.UUID]Room, 0),
		Players:  make(map[uuid.UUID]Player, 0),
		Lock:     &sync.Mutex{},
		ErrChan:  errChan,
		RecvChan: recvChan,
	}, nil
}

func (rs *RoomState) Register(addr string, udpPort int, id *uuid.UUID) (*Player, error) {
	if id == nil {
		pid := uuid.New()
		id = &pid
	}

	newPlayer := &Player{
		ID:      *id,
		Addr:    addr,
		UDPPort: udpPort,
		TCPPort: 0,
	}
	found := false
	for _, player := range rs.Players {
		if player.Addr == newPlayer.Addr {
			rs.Lock.Lock()
			rs.Players[*id] = *newPlayer
			rs.Lock.Unlock()
			found = true
		}
	}

	if !found {
		rs.Lock.Lock()
		rs.Players[*id] = *newPlayer
		rs.Lock.Unlock()
	}

	return newPlayer, nil
}

func (rs *RoomState) Join(pid *uuid.UUID, rid *uuid.UUID) (*uuid.UUID, error) {
	_, playerFound := rs.Players[*pid]
	if !playerFound {
		return nil, errors.New("client not registered")
	}

	if rid == nil {
		rid = rs.CreateRoom("")
	}

	_, roomFound := rs.Rooms[*rid]
	if !roomFound {
		return nil, errors.New("room not found")
	} else {
		room := rs.Rooms[*rid]
		player := rs.Players[*pid]

		err := room.Join(&player)
		if err != nil {
			return nil, err
		}
	}

	return rid, nil
}

func (rs *RoomState) Leave(pid *uuid.UUID, rid *uuid.UUID) (*uuid.UUID, error) {
	if pid == nil || rid == nil {
		return nil, errors.New("pid or rid null")
	}

	player, playerFound := rs.Players[*pid]
	if !playerFound {
		return nil, errors.New("client not registered")
	}

	room := rs.Rooms[*rid]
	pidLeft := room.Leave(player)
	if pidLeft == nil {
		return nil, errors.New("player to remove not found")
	}

	return pid, nil
}

func (rs RoomState) CreateRoom(name string) *uuid.UUID {
	rid := uuid.New()
	rs.Lock.Lock()
	rs.Rooms[rid] = Room{
		ID:         rid,
		Capacity:   RoomCapacity,
		Name:       name,
		NumPlayers: 0,
		Players:    make([]Player, RoomCapacity),
	}
	rs.Lock.Unlock()

	return &rid
}

func (rs *RoomState) PruneRooms() {
	rooms := make(map[uuid.UUID]Room, 0)
	for rid, room := range rs.Rooms {
		if !room.IsEmpty() {
			rooms[rid] = room
		}
	}
	rs.Lock.Lock()
	rs.Rooms = rooms
	rs.Lock.Unlock()
}

func (rs *RoomState) SendRoom(pid *uuid.UUID, rid *uuid.UUID, msg string) error {
	room, roomFound := rs.Rooms[*rid]
	if !roomFound {
		return errors.New("room not found")
	}

	if !room.InRoom(pid) {
		return errors.New("player not in room")
	}

	for _, destPlayer := range room.Players {
		destPID := destPlayer.ID
		if destPID == *pid {
			continue
		}
		err := destPlayer.SendUDP(pid, msg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (rs *RoomState) SendTo(pid *uuid.UUID, rid *uuid.UUID, to []*uuid.UUID, msg string) error {
	room, roomFound := rs.Rooms[*rid]
	if !roomFound {
		return errors.New("room not found")
	}

	if !room.InRoom(pid) {
		return errors.New("player not in room")
	}

	for _, destPlayer := range room.Players {
		inDest := false
		for _, destPID := range to {
			if *destPID == destPlayer.ID {
				inDest = true
			}
		}
		if inDest {
			err := destPlayer.SendUDP(pid, msg)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
