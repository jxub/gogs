package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net"
)

type ActionMessage struct {
	Action  string `json:"action"`
	Ident   string `json:"ident,omitempty"`
	Payload string `json:"payload,omitempty"`
}

type PayloadRegister struct {
	Addr    string
	UdpPort int
	ID      *uuid.UUID
}

type PayloadJoin struct {
	Pid *uuid.UUID
	Rid *uuid.UUID
}

type PayloadAutojoin struct {
	Pid *uuid.UUID
}

type PayloadLeave struct {
	Pid *uuid.UUID
	Rid *uuid.UUID
}

type PayloadGetRooms struct {
	Pid *uuid.UUID
}

type PayloadSend struct {
	Pid *uuid.UUID
	Rid *uuid.UUID
	Message []byte
}

type PayloadSendTo struct {
	Pid *uuid.UUID
	Rid *uuid.UUID
	RecIds []byte
	Message []byte
}

const (
	ActionRegister = "register"
	ActionJoin     = "join"
	ActionAutojoin = "autojoin"
	ActionGetRooms = "get_rooms"
	ActionCreate   = "create"
	ActionLeave    = "leave"
)

func execCommand(rs *RoomState, message *ActionMessage, conn net.Conn) error {
	switch message.Action {
	case ActionRegister:
		var msg PayloadRegister
		err := json.Unmarshal([]byte(message.Payload), &msg)
		if err != nil {
			return err
		}
		player, err := rs.Register(msg.Addr, msg.UdpPort, msg.ID)
		if err != nil {
			return err
		}
		resp := fmt.Sprintf("registered player %s\n", player.ID)
		_, err = conn.Write([]byte(resp))
		if err != nil {
			return err
		}
	case ActionJoin:
		var msg PayloadJoin
		err := json.Unmarshal([]byte(message.Payload), &msg)
		if err != nil {
			return err
		}
		rid, err := rs.Join(msg.Pid, msg.Rid)
		if err != nil {
			return err
		}
		resp := fmt.Sprintf("joined room %s\n", rid)
		_, err = conn.Write([]byte(resp))
		if err != nil {
			return err
		}
	case ActionAutojoin:
		var msg PayloadJoin
		err := json.Unmarshal([]byte(message.Payload), &msg)
		if err != nil {
			return err
		}
		rid, err := rs.Join(msg.Pid, msg.Rid)
		if err != nil {
			return err
		}
		resp := fmt.Sprintf("joined room %s\n", rid)
		_, err = conn.Write([]byte(resp))
		if err != nil {
			return err
		}
	case ActionGetRooms:
		return errors.New("action not implemented: " + message.Action)
	case ActionCreate:
		return errors.New("action not implemented: " + message.Action)
	case ActionLeave:
		return errors.New("action not implemented: " + message.Action)
	default:
		resp := fmt.Sprintf("unrecognised action: %s\n", message.Action)
		_, err := conn.Write([]byte(resp))
		if err != nil {
			return err
		}
	}

	return nil
}

func handleTCPConn(rs *RoomState, conn net.Conn) <-chan error {
	for {
		var message ActionMessage
		data, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			fmt.Println("error reading data")
			rs.ErrChan <- err
			continue
		}
		dataBytes := []byte(data)
		rs.RecvChan <- dataBytes
		err = json.Unmarshal(dataBytes, &message)
		if err != nil {
			fmt.Printf("error unmarshaling message %s\n", data)
			rs.ErrChan <- err
			continue
		}

		err = execCommand(rs, &message, conn)
		if err != nil {
			rs.ErrChan <- err
		}
	}
}

func RunUDP(rs *RoomState, port string) (<-chan []byte, <-chan error) {
	addr, err := net.ResolveUDPAddr("udp4", port)
	if err != nil {
		rs.ErrChan <- err
	}

	println("UDP addr ", addr.String())

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		rs.ErrChan <- err
	}

	defer conn.Close()
	buffer := make([]byte, 1000)

	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			rs.ErrChan <- err
		} else {
			msg := buffer[0 : n-1]
			rs.RecvChan <- msg
		}
	}
}

func RunTCP(rs *RoomState, port string) <-chan error {
	l, err := net.Listen("tcp4", port)
	if err != nil {
		rs.ErrChan <- err
		return nil
	}
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			rs.ErrChan <- err
			continue
		}
		go handleTCPConn(rs, c)
	}
}
