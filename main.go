package main

import (
	"fmt"
	"github.com/jxub/gogs/server"
	"os"
)

func main() {

	arguments := os.Args
	if len(arguments) != 3 {
		fmt.Println("Please provide tcp and udp port numbers!")
		return
	}

	tcpPort := ":" + arguments[1]
	udpPort := ":" + arguments[2]
	errChan := make(chan error)
	roomState, err := server.NewRoomState(errChan)
	if err != nil {
		panic(err)
	}

	go server.RunTCP(roomState, tcpPort)
	go server.RunUDP(roomState, udpPort)
	for {
		err := <-errChan
		println(err)
	}
}
