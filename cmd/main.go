package main

import (
	"fmt"
	"os"

	"github.com/jxub/gogs/server"
)

func main() {
	arguments := os.Args
	if len(arguments) != 3 {
		fmt.Println("Please provide tcp and udp port numbers!")
		return
	}

	tcpPort := ":" + arguments[1]
	udpPort := ":" + arguments[2]
	recvChan := make(chan []byte)
	errChan := make(chan error)
	roomState, err := server.NewRoomState(recvChan, errChan)
	if err != nil {
		panic(err)
	}

	go server.RunTCP(roomState, tcpPort)
	go server.RunUDP(roomState, udpPort)

	for {
		select {
		case errMsg := <-errChan:
			fmt.Printf("got error: %v", errMsg)
		case recvMsg := <-recvChan:
			fmt.Printf("got message: %v", string(recvMsg))
		}
	}
}
