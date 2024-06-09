package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
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
	Deployable  bool     `json:"deployable"`
}

type Player struct {
	Name        string    `json:"name"`
	PokemonList []Pokemon `json:"pokemon_list"`
}

type Participant struct {
	player     Player
	turn       int
	isWin      bool
	curPokemon Pokemon
	conn       net.Conn
}
type Message struct {
	msg  string
	conn net.Conn
}

var (
	participants []Participant
	conns        []net.Conn
	connCh       = make(chan net.Conn)
	closeCh      = make(chan Participant)
	msgCh        = make(chan string)
	msgChOne     = make(chan Message)
	starters     = []string{"Charmander", "Bulbasaur", "Squirtle"}
	mu           sync.Mutex
	writeMu      sync.Mutex
)

func main() {
	server, err := net.Listen("tcp", ":3012")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("server started")
	// Load the Pokedex
	file, _ := os.Open("./Assets/pokedex.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	pokedex := []Pokemon{}
	_ = decoder.Decode(&pokedex)
	fmt.Println("Pokedex loaded")

	go func() {
		for {
			conn, err := server.Accept()
			if err != nil {
				log.Fatal(err)
			}
			mu.Lock()
			conns = append(conns, conn)
			mu.Unlock()
			connCh <- conn
		}
	}()
	go func() {
		for {
			if len(participants) == 2 {
				winner, loser := battle(&participants[0], &participants[1])
				saveWinner(winner.player)
				msg := fmt.Sprintf("\n%s wins the battle - %s lost\n", winner.player.Name, loser.player.Name)
				msgCh <- msg + "#"
				// remove all connections
				for _, p := range participants {
					closeCh <- p
				}
			}
		}
	}()
	for {
		select {
		case conn := <-connCh:
			go onMessage(conn, pokedex)

		case msg := <-msgCh:
			fmt.Print(msg)
			publishMsgAll(msg)

		case participant := <-closeCh:
			fmt.Printf("%s exit\n", participant.player.Name)
			removeParticipant(participant)
		case msg := <-msgChOne:
			fmt.Print(msg.msg)
			publishMsgOne(msg.conn, msg.msg)
		}
	}

}

func removeParticipant(participant Participant) {

	for i := range participants {
		if participants[i].player.Name == participant.player.Name {
			participants = append(participants[:i], participants[i+1:]...)
			break
		}
	}
	// Remove from conns
	for i, conn := range conns {
		if conn == participant.conn {
			conns = append(conns[:i], conns[i+1:]...)
			break
		}
	}
}

// func publishMsgExcept(conn net.Conn, msg string) {
// 	for i := range conns {
// 		if conns[i] != conn {
// 			writer := bufio.NewWriter(conns[i])
// 			writer.WriteString(msg)
// 			writer.Flush()
// 		}
// 	}
// }

func publishMsgOne(conn net.Conn, msg string) error {
	writeMu.Lock()
	defer writeMu.Unlock()

	msgBytes := []byte(msg)
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, uint32(len(msgBytes)))

	if _, err := conn.Write(lenBytes); err != nil {
		return err
	}
	if _, err := conn.Write(msgBytes); err != nil {
		return err
	}
	return nil
}

func publishMsgAll(msg string) error {
	writeMu.Lock()
	defer writeMu.Unlock()

	for i := range conns {
		msgBytes := []byte(msg)
		lenBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBytes, uint32(len(msgBytes)))

		if _, err := conns[i].Write(lenBytes); err != nil {
			return err
		}
		if _, err := conns[i].Write(msgBytes); err != nil {
			return err
		}
	}
	return nil
}

func onMessage(conn net.Conn, pokedex []Pokemon) {
	fmt.Println("new client")
	reader := bufio.NewReader(conn)
	playerName, err := reader.ReadString('\n')
	playerName = strings.TrimSpace(playerName)
	if err != nil {
		return
	}
	fmt.Println(playerName)
	player, found := findPlayer(playerName)
	if !found {
		publishMsgOne(conn, "Player does not exist. Created a new player.\n#")
		player = createPlayer(pokedex, playerName)
	}
	// request the player to choose a Pokemon
	// make all the Pokemon deployable
	for i := range player.PokemonList {
		player.PokemonList[i].Deployable = true
	}
	msg := listPokemon(player.PokemonList)

	chosenPokemon, _ := readPokemonFromClient(conn, msg[:len(msg)-1]+"Choose a pokemon: #", player.PokemonList)

	// Add the player to the list of participants
	mu.Lock()
	participants = append(participants, Participant{
		player:     player,
		turn:       3,
		isWin:      false,
		curPokemon: chosenPokemon,
		conn:       conn,
	})
	mu.Unlock()
	fmt.Println(len(participants))

}

func createPlayer(pokedex []Pokemon, playerName string) Player {
	// Create the player

	player := Player{
		Name:        playerName,
		PokemonList: []Pokemon{},
	}
	// Choose 3 starter Pokemon
	for _, p := range starters {
		pokemon, _ := findPokemon(pokedex, p)
		player.PokemonList = append(player.PokemonList, pokemon)
	}
	// Load the existing players from the JSON file
	file, _ := os.Open("./Assets/players.json")
	decoder := json.NewDecoder(file)
	existingPlayers := []Player{}
	_ = decoder.Decode(&existingPlayers)
	file.Close()

	// Append the new player to the list
	existingPlayers = append(existingPlayers, player)

	// Save the updated list of players to the JSON file
	file, _ = os.Create("./Assets/players.json")
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ") // Set indent to 4 spaces
	_ = encoder.Encode(existingPlayers)

	fmt.Printf("Player %s created\n", playerName)
	return player
}

// func contains(s []string, str string) bool {
// 	for _, v := range s {
// 		if strings.EqualFold(v, str) {
// 			return true
// 		}
// 	}
// 	return false
// }

func findPokemon(pokedex []Pokemon, name string) (Pokemon, bool) {
	for _, p := range pokedex {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Pokemon{}, false
}
func findPlayer(name string) (Player, bool) {
	// Load the Pokedex
	file, _ := os.Open("./Assets/players.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	players := []Player{}
	_ = decoder.Decode(&players)
	for _, p := range players {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return Player{}, false
}
func calculateDamage(attacker, defender *Pokemon, attackType string) int {
	if attackType == "normal" {
		return max(attacker.Attack-defender.Defense, 0)
	}
	if attackType == "special" {
		elementalMultiplier := 1.75
		return max(int(float64(attacker.SpAttack)*elementalMultiplier)-defender.SpDefense, 0)
	}
	return 0
}

// Helper function to get the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Battle function
func battle(participant1, participant2 *Participant) (*Participant, *Participant) {
	var surrendered bool
	for {

		// Start the battle
		winner, loser := battleRound(participant1, participant2)
		msg := fmt.Sprintf("\n%s wins the round - %s lost", winner.player.Name, loser.player.Name)

		msgCh <- msg + "#"
		if loser.turn == 0 {
			msg := fmt.Sprintf("\nBATTLE END!!! \n%s has no turns left. %s wins!", loser.player.Name, winner.player.Name)

			msgCh <- msg + "#"
			totalExp := 0
			for _, pokemon := range loser.player.PokemonList {
				totalExp += pokemon.Exp
			}

			// Distribute the total experience to the winning team
			expPerPokemon := totalExp / 3
			for i := range winner.player.PokemonList {
				winner.player.PokemonList[i].Exp += expPerPokemon
			}
			return winner, loser

		}
		// Ask the losing participant to choose another Pokemon
		pokeList := listPokemon(loser.player.PokemonList)

		loser.curPokemon, surrendered = readPokemonFromClient(loser.conn, "\nYou lost the round, Let's choose another Pokemon\n"+pokeList+"Your choice: ", loser.player.PokemonList)
		if surrendered {
			totalExp := 0
			for _, pokemon := range loser.player.PokemonList {
				totalExp += pokemon.Exp
			}

			// Distribute the total experience to the winning team
			expPerPokemon := totalExp / 3
			for i := range winner.player.PokemonList {
				winner.player.PokemonList[i].Exp += expPerPokemon
			}
			return winner, loser
		}
	}
}
func battleRound(participant1, participant2 *Participant) (*Participant, *Participant) {
	var messages []string
	// Announce the current Pokemon
	msg := fmt.Sprintf("---%s chose %s\n%s chose %s\n", participant1.player.Name, participant1.curPokemon.Name, participant2.player.Name, participant2.curPokemon.Name)

	messages = append(messages, msg)
	messages = append(messages, "------------BATTLE REPORT------------\n")

	var winner, loser *Participant
	var attacker, defender *Participant
	if participant1.curPokemon.Speed > participant2.curPokemon.Speed {
		attacker = participant1
		defender = participant2
	} else if participant1.curPokemon.Speed < participant2.curPokemon.Speed {
		attacker = participant2
		defender = participant1
	} else {
		// If the speeds are equal, randomly choose the attacker
		if rand.Intn(2) == 0 {
			attacker = participant1
			defender = participant2
		} else {
			attacker = participant2
			defender = participant1
		}
	}
	for participant1.curPokemon.HP > 0 && participant2.curPokemon.HP > 0 {

		// Player's turn to choose attack type
		attackTypes := []string{"normal", "special"}
		attackType := attackTypes[rand.Intn(len(attackTypes))]

		// Calculate and apply damage
		dmg := calculateDamage(&attacker.curPokemon, &defender.curPokemon, attackType)
		defender.curPokemon.HP -= dmg
		msg := fmt.Sprintf("%s attacked %s with %s attack dealing %d damage.\n", attacker.curPokemon.Name, defender.curPokemon.Name, attackType, dmg)
		messages = append(messages, msg)
		if defender.curPokemon.HP <= 0 {
			msg := fmt.Sprintf("%s fainted.\n", defender.curPokemon.Name)
			defender.turn--
			for i := range defender.player.PokemonList {
				if defender.player.PokemonList[i].Name == defender.curPokemon.Name {
					// update hp to 0
					defender.player.PokemonList[i].HP = 0
					// update deployable
					defender.player.PokemonList[i].Deployable = false
				} else {
					// update hp winner

					attacker.player.PokemonList[i].HP = attacker.curPokemon.HP
				}
			}

			loser = defender
			winner = attacker
			messages = append(messages, msg)
			break
		}

		// Swap attacker and defender for the next round
		attacker, defender = defender, attacker
	}
	msg = fmt.Sprintf("%s has %d HP left.\n", attacker.curPokemon.Name, attacker.curPokemon.HP)
	messages = append(messages, msg)
	// announce the turns
	msg = fmt.Sprintf("%s has %d turns left.\n", participant1.player.Name, participant1.turn)
	messages = append(messages, msg)
	msg = fmt.Sprintf("%s has %d turns left.\n", participant2.player.Name, participant2.turn)
	messages = append(messages, msg)
	messages = append(messages, "------END BATTLE REPORT-----")

	msgCh <- strings.Join(messages, "") + "#"

	return winner, loser
}
func readPokemonFromClient(conn net.Conn, msg string, pokemonList []Pokemon) (Pokemon, bool) {
	// conn.Write([]byte(msg))
	var chosenPokemon Pokemon
	msgChOne <- Message{msg: msg, conn: conn}

	for {
		reader := bufio.NewReader(conn)
		pokemonIndex, err := reader.ReadString('\n')

		if err != nil {
			return Pokemon{}, true
		}
		pokemonIndex = strings.TrimSpace(pokemonIndex)
		index, _ := strconv.Atoi(pokemonIndex)

		// Check if the player wants to surrender
		if index == -1 {
			return Pokemon{}, true
		} else {
			chosenPokemon = pokemonList[index-1]
		}
		// Check if the chosen Pokemon is deployable
		if chosenPokemon.Deployable {
			return chosenPokemon, false
		} else if !chosenPokemon.Deployable {

			// If the Pokemon is not deployable, send a message to the client and ask for another Pokemon
			msg := "This Pokemon lost the ability to fight. Please choose another one.\n#"
			conn.Write([]byte(msg))
		}
	}

}
func listPokemon(pokemonList []Pokemon) string {
	var listOfPokemon []string
	for i, p := range pokemonList {
		listOfPokemon = append(listOfPokemon, fmt.Sprintf("%d. %s\n", i+1, p.Name))
	}
	return strings.Join(listOfPokemon, "") + "#"
}
func saveWinner(player Player) {
	// Load the existing players from the JSON file
	file, _ := os.Open("./Assets/players.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	existingPlayers := []Player{}
	_ = decoder.Decode(&existingPlayers)

	// Find the winner in the list and update their data
	for i, existingPlayer := range existingPlayers {
		if existingPlayer.Name == player.Name {
			existingPlayers[i] = player
			break
		}
	}

	// Save the updated list of players to the JSON file
	file, _ = os.Create("./Assets/players.json")
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ") // Set indent to 4 spaces
	_ = encoder.Encode(existingPlayers)

	fmt.Printf("Winner %s saved\n", player.Name)
}
