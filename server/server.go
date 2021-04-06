package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"math/rand"
	"net"
	"strings"
	"time"
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

const (
	ActionRegister   = "register"
	ActionJoin       = "join"
	ACTION_AUTOJOIN  = "autojoin"
	ACTION_GET_ROOMS = "get_rooms"
	ACTION_CREATE    = "create"
	ACTION_LEAVE     = "leave"
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

		err = json.Unmarshal([]byte(data), &message)
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

func RunUDP(rs *RoomState, port string) <-chan error {
	addr, err := net.ResolveUDPAddr("udp4", port)
	if err != nil {
		rs.ErrChan <- err
	}

	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		rs.ErrChan <- err
	}

	defer conn.Close()
	buffer := make([]byte, 10)
	rand.Seed(time.Now().Unix())

	for {
		n, addr, err := conn.ReadFromUDP(buffer)
		fmt.Print("-> ", string(buffer[0:n-1]))

		if strings.TrimSpace(string(buffer[0:n])) == "STOP" {
			fmt.Println("Exiting UDP server!")
			rs.ErrChan <- err
		}
		data := []byte("aaa")
		_, err = conn.WriteToUDP(data, addr)
		if err != nil {
			rs.ErrChan <- err
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
