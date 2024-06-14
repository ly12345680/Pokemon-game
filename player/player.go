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

var (
	consoleLock sync.Mutex
)

func onMessage(conn net.Conn) {
	reader := bufio.NewReader(conn)
	for {
		msg, _ := reader.ReadString('#')

		consoleLock.Lock()
		fmt.Print(msg[:len(msg)-1])
		consoleLock.Unlock()
	}
}

func main() {
	connection, err := net.Dial("tcp", "localhost:3015")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("MODE: 1. POKEBAT \t 2. POKECAT\nType following syntax: [Name] [Mode]\nYour Input: ")
	nameReader := bufio.NewReader(os.Stdin)
	input, _ := nameReader.ReadString('\n')

	connection.Write([]byte(input))
	fmt.Println("********** Entered Game **********")

	go onMessage(connection)

	// separate the name and mode
	input = strings.TrimSpace(input)
	// playerName := strings.Split(input, " ")[0]
	mode := strings.Split(input, " ")[1]
	if mode == "1" {
		for {
			msgReader := bufio.NewReader(os.Stdin)

			msg, err := msgReader.ReadString('\n')
			// Remove any CR characters from the input
			msg = strings.Replace(msg, "\r", "", -1)
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
				msg := string(rune(event.Key))
				consoleLock.Lock()
				_, err := connection.Write([]byte(msg + "\n"))
				consoleLock.Unlock()
				// fmt.Println("Sent key press event to server: ", string(event.Key))
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
