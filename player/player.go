package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/eiannone/keyboard"
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

var (
	consoleLock sync.Mutex
)

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

	fmt.Print("MODE: 1. POKEBAT \t 2. POKECAT\nType following syntax: [Name] [Mode]\nYour Input: ")
	nameReader := bufio.NewReader(os.Stdin)
	input, _ := nameReader.ReadString('\n')

	connection.Write([]byte(input))
	fmt.Println("********** MESSAGES **********")

	go onMessage(connection)

	// separate the name and mode
	input = strings.TrimSpace(input)
	// playerName := strings.Split(input, " ")[0]
	mode := strings.Split(input, " ")[1]
	if mode == "1" {
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
	} else if mode == "2" {
		// go onKeyInput(connection, inputCh)

		for {
			keysEvents, err := keyboard.GetKeys(10)
			if err != nil {
				panic(err)
			}
			for {
				event := <-keysEvents
				if event.Err != nil {
					panic(event.Err)
				}
				// Send the key character to the input channel
				msg := string(event.Key)
				consoleLock.Lock()
				_, err := connection.Write([]byte(msg + "\n"))
				consoleLock.Unlock()
				fmt.Println("Sent key press event to server: ", string(event.Key))
				if err != nil {
					fmt.Println("Failed to send key press event to server:", err)
					return
				}
				// if event.Key == keyboard.KeyEsc {

				// }
			}
		}
	}
	// connection.Close()
}
