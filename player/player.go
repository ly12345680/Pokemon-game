package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
)

type Pokemon struct {
	Index       string   `json:"index"`
	Name        string   `json:"name"`
	Exp         int      `json:"exp"`
	HP          int      `json:"hp"`
	Attack      int      `json:"attack"`
	Defense     int      `json:"defense"`
	SpAttack    int      `json:"sp_attack"`
	SpDefense   int      `json:"sp_defense"`
	Speed       int      `json:"speed"`
	TotalEVs    int      `json:"total_evs"`
	Type        []string `json:"type"`
	Description string   `json:"description"`
	Height      string   `json:"height"`
	Weight      string   `json:"weight"`
	ImageURL    string   `json:"image_url"`
	Level       int      `json:"level"`
	AccumExp    int      `json:"accum_exp"`
}

type Player struct {
	Name        string    `json:"name"`
	PokemonList []Pokemon `json:"pokemon_list"`
}

var consoleLock sync.Mutex

func onMessage(conn net.Conn) {
	for {
		reader := bufio.NewReader(conn)
		msg, _ := reader.ReadString('#')

		consoleLock.Lock()
		fmt.Print(msg[:len(msg)-1])
		consoleLock.Unlock()
	}
}

func main() {
	connection, err := net.Dial("tcp", "localhost:3012")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("your name:")
	nameReader := bufio.NewReader(os.Stdin)
	nameInput, _ := nameReader.ReadString('\n')

	// nameInput = nameInput[:len(nameInput)-1]

	connection.Write([]byte(nameInput))
	fmt.Println("********** MESSAGES **********")

	go onMessage(connection)

	for {
		msgReader := bufio.NewReader(os.Stdin)
		msg, err := msgReader.ReadString('\n')
		if err != nil {
			break
		}

		consoleLock.Lock()
		_, err = connection.Write([]byte(msg))
		if err != nil {
			fmt.Println("Connection closed")
			break
		}
		consoleLock.Unlock()
	}

	connection.Close()
}
