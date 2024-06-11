package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
)

var (
	pokedex []Pokemon
)

const (
	worldSize       = 25
	spawTime        = 5 * time.Second
	despawnTime     = 10 * time.Second
	pokemonPerSpawn = 10
	playerLink      = "./server/Assets/players.json"
	maxPokemon      = 10
)

var (
	endCh = make(chan string)
)

type Position struct {
	X, Y int
}

type Player struct {
	Name     string     `json:"name"`
	PokeList []*Pokemon `json:"pokemon_list"`
	pos      Position
}

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
}

type World struct {
	size     int
	grid     [][]interface{}
	players  map[string]*Player
	pokemons []*Pokemon
	mux      sync.Mutex
}

func newWorld(size int) *World {
	grid := make([][]interface{}, size)
	for i := range grid {
		grid[i] = make([]interface{}, size)
	}
	return &World{
		size:     size,
		grid:     grid,
		players:  make(map[string]*Player),
		pokemons: []*Pokemon{},
	}
}

func (w *World) addPlayer(name string, x int, y int) {
	w.mux.Lock()
	defer w.mux.Unlock()
	pos := Position{x, y}
	// get pokelist from json file
	file, _ := os.Open(playerLink)
	defer file.Close()
	decoder := json.NewDecoder(file)
	existingPlayers := []Player{}
	_ = decoder.Decode(&existingPlayers)
	for _, existingPlayer := range existingPlayers {
		if existingPlayer.Name == name {
			w.players[name] = &Player{Name: name, PokeList: existingPlayer.PokeList, pos: pos}
			w.grid[pos.X][pos.Y] = w.players[name]
			return
		}
	}
	// if player is not in the json file
	player := &Player{Name: name, pos: pos, PokeList: []*Pokemon{}}
	w.players[name] = player
	w.grid[pos.X][pos.Y] = player
}

func (w *World) movePlayer(name string, dx, dy int) {
	w.mux.Lock()
	defer w.mux.Unlock()
	player, ok := w.players[name]
	if !ok {
		return
	}
	w.grid[player.pos.X][player.pos.Y] = nil
	x := (player.pos.X + dx + w.size) % w.size
	y := (player.pos.Y + dy + w.size) % w.size
	// Check if there's a Pokémon at the new position
	if p, ok := w.grid[x][y].(*Pokemon); ok {
		if len(player.PokeList) <= maxPokemon {
			fmt.Printf("%s captured %s!\n", player.Name, p.Name)
			player.PokeList = append(player.PokeList, p)
			w.removePokemon(p)
			// Remove player from old position
			savePlayerData(*player)
			time.Sleep(2 * time.Second)
		} else {
			x = (player.pos.X + w.size) % w.size
			y = (player.pos.Y + w.size) % w.size
			fmt.Println("You have reached the maximum number of Pokémon. You can't capture more.")
			time.Sleep(1 * time.Second)
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
					fmt.Print("🚶")
				case *Pokemon:
					fmt.Print("🐤")
				}
			} else {
				fmt.Print(". ")
			}
		}
		fmt.Println()
	}
	fmt.Println()
}
func (w *World) spawnPokemonWave() {
	for i := 0; i < pokemonPerSpawn; i++ {
		// Generate random x and y coordinates for the Pokemon
		x := rand.Intn(w.size)
		y := rand.Intn(w.size)

		pokemon := &pokedex[rand.Intn(len(pokedex))]
		// Create a new Pokemon
		pos := Position{x, y}
		pokemon.pos = pos
		pokemon.spawnTime = time.Now()

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
func savePlayerData(player Player) {
	// Load the existing players from the JSON file
	file, _ := os.Open(playerLink)
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

func main() {
	// Load the Pokedex
	file, _ := os.Open("./server/Assets/pokedex.json")
	defer file.Close()
	decoder := json.NewDecoder(file)
	pokedex = []Pokemon{}
	_ = decoder.Decode(&pokedex)
	fmt.Println("Pokedex loaded")
	name := getPlayerName()

	rand.Seed(time.Now().UnixNano())

	world := newWorld(worldSize)

	// Adding players
	world.addPlayer(name, 0, 1)

	inputCh := make(chan string)

	// Start player input goroutine
	go func() {
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
			inputCh <- string(event.Key)
		}
	}()

	// Start moving players
	go func() {

	gameLoop:
		for {
			input := <-inputCh
			switch input {
			case string(keyboard.KeyArrowUp):
				world.movePlayer(name, -1, 0)
				world.display()
			case string(keyboard.KeyArrowDown):
				world.movePlayer(name, 1, 0)
				world.display()
			case string(keyboard.KeyArrowLeft):
				world.movePlayer(name, 0, -1)
				world.display()
			case string(keyboard.KeyArrowRight):
				world.movePlayer(name, 0, 1)
				world.display()
			case string(keyboard.KeyEsc):
				endCh <- "end"
				break gameLoop
			}

		}

	}()

	// Spawn Pokémon randomly every few seconds
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
