package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
	Deployable  bool     `json:"deployable"`
	EVPoints    float64
	pos         Position
	spawnTime   time.Time
	avatar      string
}

type Player struct {
	Name        string     `json:"name"`
	PokemonList []*Pokemon `json:"pokemon_list"`
	pos         Position
	avatar      string
}
type World struct {
	size     int
	grid     [][]interface{}
	players  map[string]*Player
	pokemons []*Pokemon
	mux      sync.Mutex
}
type Participant struct {
	player     *Player
	turn       int
	isWin      bool
	curPokemon Pokemon
	conn       net.Conn
	catchMode  bool
}
type Message struct {
	msg  string
	conn net.Conn
}
type Position struct {
	X, Y int
}

const (
	worldSize       = 25
	spawTime        = 5 * time.Second
	despawnTime     = 10 * time.Second
	pokemonPerSpawn = 10
	playerLink      = "./Assets/players.json"
	maxPokemon      = 10
)

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
	endCh        = make(chan string)
	pokedex      []Pokemon
	// moveCh        = make(chan string)
	world         = newWorld(worldSize)
	avatarPokeman = []string{"🏃", "🙎", "🧛", "🥷", "👨", "🚶"}
	avatarPokemon = []string{"🔥", "🌿", "💧", "⛰️", "🪽", "⚡️"}
)

func main() {
	// Create the world

	// Start the server
	server, err := net.Listen("tcp", ":3012")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("server started")
	// Load the Pokedex
	file, _ := os.Open("./Assets/pokedex.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	_ = decoder.Decode(&pokedex)
	// fmt.Println(pokedex)
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
			batleModeCount := 0
			for _, p := range participants {
				if !p.catchMode {
					batleModeCount++
				}
			}
			if batleModeCount == 2 {
				winner, loser := battle(&participants[0], &participants[1])
				saveWinner(winner.player)
				msg := fmt.Sprintf("\n%s wins the battle - %s lost\n", winner.player.Name, loser.player.Name)
				msgCh <- msg + "#"
				// remove all connections
				for _, p := range participants {
					if !p.catchMode {
						closeCh <- p
					}
				}
			}
		}
	}()
	go func() {
		// check if there are any players in the game

		for {
			time.Sleep(spawTime)
			// check if there are any players in the game
			isCatchMode := false
			for _, p := range participants {
				if p.catchMode {
					isCatchMode = true
				}
			}
			// Spawn Pokémon randomly every few seconds
			if isCatchMode {
				go func() {
					for {
						time.Sleep(spawTime)
						world.spawnPokemonWave()
						world.display()
					}
				}()

				// Despawn Pokémon every minute
				go func() {
					for {
						time.Sleep(despawnTime)
						world.deSpawnPokemons()

						world.display()
					}
				}()
				// Wait for the game to end
				end := <-endCh
				fmt.Printf("Game %s\n", end)
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
			// remove player from the world
			fmt.Println(participant.catchMode)
			if participant.catchMode {
				world.mux.Lock()
				delete(world.players, participant.player.Name)
				world.grid[participant.player.pos.X][participant.player.pos.Y] = nil
				world.mux.Unlock()
			}

		case msg := <-msgChOne:
			fmt.Print(msg.msg)
			publishMsgOne(msg.conn, msg.msg)
		}
	}

}
func newWorld(size int) *World {
	grid := make([][]interface{}, size)
	for i := range grid {
		grid[i] = make([]interface{}, size)
	}
	// spawn pokemon

	return &World{
		size:     size,
		grid:     grid,
		players:  make(map[string]*Player),
		pokemons: []*Pokemon{},
	}
}
func (w *World) addPlayer(name string, x int, y int) *Player {
	w.mux.Lock()
	defer w.mux.Unlock()
	pos := Position{x, y}
	playerAvatar := avatarPokeman[len(avatarPokeman)-1]
	avatarPokeman = avatarPokeman[:len(avatarPokeman)-1]
	// Check if there's another player at the initial position
	for {
		if w.grid[pos.X][pos.Y] != nil {
			pos.X = (pos.X + 1) % w.size
			pos.Y = (pos.Y + 1) % w.size
		} else {
			break
		}
	}

	// get pokemonlist from json file
	file, _ := os.Open(playerLink)
	defer file.Close()
	decoder := json.NewDecoder(file)
	existingPlayers := []Player{}
	_ = decoder.Decode(&existingPlayers)
	for _, existingPlayer := range existingPlayers {
		if existingPlayer.Name == name {
			w.players[name] = &Player{Name: name, PokemonList: existingPlayer.PokemonList, pos: pos, avatar: playerAvatar}
			w.grid[pos.X][pos.Y] = w.players[name]
			return w.players[name]
		}
	}
	// if player is not in the json file
	player := &Player{Name: name, pos: pos, PokemonList: []*Pokemon{}, avatar: playerAvatar}
	w.players[name] = player
	w.grid[pos.X][pos.Y] = player
	return player
}
func (w *World) movePlayer(name string, dx, dy int) {
	w.mux.Lock()
	defer w.mux.Unlock()
	player, ok := w.players[name]
	if !ok {
		return
	}
	// Save the player's old position
	oldX := (player.pos.X + w.size) % w.size
	oldY := (player.pos.Y + w.size) % w.size
	newX := (player.pos.X + dx + w.size) % w.size
	newY := (player.pos.Y + dy + w.size) % w.size
	// Move forward
	w.grid[player.pos.X][player.pos.Y] = nil
	x := newX
	y := newY
	// Check if there's another player at the new position
	if _, ok := w.grid[x][y].(*Player); ok {
		x = oldX
		y = oldY
		fmt.Println("There's another player at the new position. You can't move there.")
		time.Sleep(2 * time.Second)
	}
	// Check if there's a Pokémon at the new position
	if p, ok := w.grid[x][y].(*Pokemon); ok {
		if len(player.PokemonList) <= maxPokemon {
			fmt.Printf("%s captured %s!\n", player.Name, p.Name)
			player.PokemonList = append(player.PokemonList, p)
			w.removePokemon(p)
			// Remove player from old position
			savePlayerData(*player)
			time.Sleep(2 * time.Second)
		} else {
			x = oldX
			y = oldY
			fmt.Println("You have reached the maximum number of Pokémon. You can't capture more.")
			time.Sleep(2 * time.Second)
		}
	}

	// Update player position
	player.pos.X = x
	player.pos.Y = y

	// Place player in new position
	w.grid[player.pos.X][player.pos.Y] = player
}

func (w *World) display() {
	w.mux.Lock()
	defer w.mux.Unlock()
	fmt.Println("\n\n\n")

	for i := 0; i < w.size; i++ {
		for j := 0; j < w.size; j++ {
			if w.grid[i][j] != nil {
				switch w.grid[i][j].(type) {
				case *Player:
					fmt.Print(w.grid[i][j].(*Player).avatar)
				case *Pokemon:
					fmt.Print(w.grid[i][j].(*Pokemon).avatar)
				}
			} else {
				fmt.Print("￭ ")
			}
		}
		fmt.Println()
	}
	fmt.Println()
}
func (w *World) spawnPokemonWave() {
	// Ensure n is greater than 0
	n := len(pokedex)
	if n <= 0 {
		fmt.Println("No pokemons to spawn")
		return
	}
	for i := 0; i < pokemonPerSpawn; i++ {
		// Generate random x and y coordinates for the Pokemon
		x := rand.Intn(w.size)
		y := rand.Intn(w.size)
		// check if there's another entity at the initial position
		for {
			if w.grid[x][y] != nil {
				x = (x + 1) % w.size
				y = (y + 1) % w.size
			} else {
				break
			}
		}
		pokemon := &pokedex[rand.Intn(len(pokedex))]
		// Create a new Pokemon
		pos := Position{x, y}
		pokemon.pos = pos
		pokemon.spawnTime = time.Now()
		// Randomly generate the EV points
		pokemon.EVPoints = math.Round((0.5+rand.Float64()/2)*100) / 100
		// assign avatar
		switch pokemon.Type[0] {
		case "fire":
			pokemon.avatar = avatarPokemon[0]
		case "grass":
			pokemon.avatar = avatarPokemon[1]
		case "water":
			pokemon.avatar = avatarPokemon[2]
		case "rock":
			pokemon.avatar = avatarPokemon[3]
		case "Flying":
			pokemon.avatar = avatarPokemon[4]
		case "electric":
			pokemon.avatar = avatarPokemon[5]
		}

		// Add the Pokemon to the world
		w.grid[x][y] = pokemon
		w.pokemons = append(w.pokemons, pokemon)
	}
}

func (w *World) removePokemon(p *Pokemon) {
	for i, pokemon := range w.pokemons {
		if pokemon == p {
			w.pokemons = append(w.pokemons[:i], w.pokemons[i+1:]...)
			break
		}
	}
	w.grid[p.pos.X][p.pos.Y] = nil
}
func getPlayerName() string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Press ESC to cancel Game\nEnter your name: ")
	name, _ := reader.ReadString('\n')
	return strings.TrimSpace(name)
}
func getInput(conn net.Conn) string {
	// fmt.Print("Press ESC to cancel Game\nEnter your name: ")
	mu.Lock()
	reader := bufio.NewReader(conn)
	input, _ := reader.ReadString('\n')
	mu.Unlock()

	return strings.TrimSpace(input)
}
func savePlayerData(player Player) {
	// Load the existing players from the JSON file
	file, _ := os.Open(playerLink)
	defer file.Close()
	decoder := json.NewDecoder(file)
	existingPlayers := []Player{}
	_ = decoder.Decode(&existingPlayers)
	// condition to check new player
	isNewPlayer := false
	// Find the existed player in the list and update their data
	for i, existingPlayer := range existingPlayers {
		if existingPlayer.Name == player.Name {
			existingPlayers[i] = player
			break
		}
	}
	// Save new player data to the JSON file
	if !isNewPlayer {
		existingPlayers = append(existingPlayers, player)
	}
	file, _ = os.Create(playerLink)
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ") // Set indent to 4 spaces
	_ = encoder.Encode(existingPlayers)

	fmt.Printf("Player %s saved\n", player.Name)

}
func (w *World) deSpawnPokemons() {
	w.mux.Lock()
	defer w.mux.Unlock()
	now := time.Now()
	for _, p := range w.pokemons {
		if now.Sub(p.spawnTime) >= despawnTime-time.Second*2 {
			// fmt.Printf("%s despawned\n", p.Name)
			w.removePokemon(p)
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
	fmt.Println("A client connected")
	reader := bufio.NewReader(conn)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal(err)
	}
	// separate the name and mode
	input = strings.TrimSpace(input)
	playerName := strings.Split(input, " ")[0]
	mode := strings.Split(input, " ")[1]

	// mode = 1 for battle mode
	// mode = 2 for catch mode
	if mode == "1" {
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
			catchMode:  false,
		})
		mu.Unlock()
		fmt.Println("The number of connected participants: ", len(participants))

	} else if mode == "2" {

		// create a new player
		playerName = strings.TrimSpace(playerName)
		if err != nil {
			return
		}
		fmt.Println(playerName)
		player := world.addPlayer(playerName, 0, 0)
		// Add the player to the list of participants
		mu.Lock()
		participants = append(participants, Participant{
			player:    player,
			conn:      conn,
			catchMode: true,
		})
		mu.Unlock()
		fmt.Println("The number of connected participants: ", len(participants))
		go handlePlayerMovement(conn, world, playerName)
	}
}

func createPlayer(pokedex []Pokemon, playerName string) *Player {
	// Create the player

	player := Player{
		Name:        playerName,
		PokemonList: []*Pokemon{},
	}
	// Choose 3 starter Pokemon
	for _, p := range starters {
		pokemon, _ := findPokemon(pokedex, p)
		player.PokemonList = append(player.PokemonList, &pokemon)
	}
	// Load the existing players from the JSON file
	file, _ := os.Open(playerLink)
	decoder := json.NewDecoder(file)
	existingPlayers := []Player{}
	_ = decoder.Decode(&existingPlayers)
	file.Close()

	// Append the new player to the list
	existingPlayers = append(existingPlayers, player)

	// Save the updated list of players to the JSON file
	file, _ = os.Create(playerLink)
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ") // Set indent to 4 spaces
	_ = encoder.Encode(existingPlayers)

	fmt.Printf("Player %s created\n", playerName)
	return &player
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
func findPlayer(name string) (*Player, bool) {
	// Load the Pokedex
	file, _ := os.Open(playerLink)
	defer file.Close()
	decoder := json.NewDecoder(file)
	players := []Player{}
	_ = decoder.Decode(&players)
	for _, p := range players {
		if strings.EqualFold(p.Name, name) {
			return &p, true
		}
	}
	return &Player{}, false
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
func handlePlayerMovement(conn net.Conn, world *World, playerName string) {
	fmt.Println("Player movement handler started")
	reader := bufio.NewReader(conn)
	go func() {
		for {
			input, err := reader.ReadString('\n')
			if err != nil {
				log.Fatal(err)
			}
			input = strings.TrimSpace(input)
			switch input {
			case string(keyboard.KeyArrowUp):
				world.movePlayer(playerName, -1, 0)
			case string(keyboard.KeyArrowDown):
				world.movePlayer(playerName, 1, 0)
			case string(keyboard.KeyArrowLeft):
				world.movePlayer(playerName, 0, -1)
			case string(keyboard.KeyArrowRight):
				world.movePlayer(playerName, 0, 1)
			case string(keyboard.KeyEsc):
				endCh <- "ended"
				for _, p := range participants {
					if p.player.Name == playerName {
						closeCh <- p
					}
				}
				return
			}
			world.display()
		}
	}()

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
				winner.player.PokemonList[i].AccumExp += expPerPokemon
			}
			return winner, loser

		}
		// Ask the losing participant to choose another Pokemon
		pokemonList := listPokemon(loser.player.PokemonList)

		loser.curPokemon, surrendered = readPokemonFromClient(loser.conn, "\nYou lost the round, Let's choose another Pokemon\n"+pokemonList+"Your choice: ", loser.player.PokemonList)
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
func readPokemonFromClient(conn net.Conn, msg string, pokemonList []*Pokemon) (Pokemon, bool) {
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
			chosenPokemon = *pokemonList[index-1]
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
func listPokemon(pokemonList []*Pokemon) string {
	var listOfPokemon []string
	for i, p := range pokemonList {
		listOfPokemon = append(listOfPokemon, fmt.Sprintf("%d. %s\n", i+1, p.Name))
	}
	return strings.Join(listOfPokemon, "") + "#"
}
func saveWinner(player *Player) {
	// Load the existing players from the JSON file
	file, _ := os.Open(playerLink)
	defer file.Close()
	decoder := json.NewDecoder(file)
	existingPlayers := []Player{}
	_ = decoder.Decode(&existingPlayers)

	// Find the winner in the list and update their data
	for i, existingPlayer := range existingPlayers {
		if existingPlayer.Name == player.Name {
			existingPlayers[i] = *player
			break
		}
	}

	// Save the updated list of players to the JSON file
	file, _ = os.Create(playerLink)
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "    ") // Set indent to 4 spaces
	_ = encoder.Encode(existingPlayers)

	fmt.Printf("Winner %s saved\n", player.Name)
}
