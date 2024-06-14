package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/chromedp/chromedp"
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
}

func fetchMainPageHTML(ctx context.Context) (string, error) {
	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate("https://pokedex.org/"),
		chromedp.Sleep(1*time.Microsecond),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load main page: %v", err)
	}
	return html, nil
}

func fetchPokemonPageHTML(ctx context.Context, url string) (string, error) {
	var html string
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(1*time.Microsecond),
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return "", fmt.Errorf("failed to load Pokémon page: %v", err)
	}
	return html, nil
}

func parseMainPage(html string) ([]string, []string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse main page HTML: %v", err)
	}

	var urls []string
	var names []string
	doc.Find("#monsters-list-wrapper li").Each(func(i int, s *goquery.Selection) {
		fmt.Println(s.Contents().Text())
		if i >= 649 {
			return
		}
		name := strings.TrimSpace(s.Find("span").Text())
		classAttr, exists := s.Find("button").Attr("class")
		id := ""
		if exists {
			id = strings.TrimPrefix(classAttr, "monster-sprite sprite-")
		}
		url := fmt.Sprintf("https://pokedex.org/#/pokemon/%s", id)
		names = append(names, name)
		urls = append(urls, url)
	})
	return names, urls, nil
}

func parsePokemonPage(html, name, index string) (Pokemon, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return Pokemon{}, fmt.Errorf("failed to parse Pokémon page HTML: %v", err)
	}

	pokemon := Pokemon{Index: index, Name: name, Type: []string{}}

	doc.Find(".detail-types .monster-type").Each(func(i int, s *goquery.Selection) {
		pokemon.Type = append(pokemon.Type, strings.TrimSpace(s.Text()))
	})

	doc.Find(".detail-stats-row").Each(func(i int, s *goquery.Selection) {
		statName := strings.ToUpper(strings.TrimSpace(s.Find("span").First().Text()))
		statValueStr := strings.TrimSpace(s.Find(".stat-bar-fg").Text())
		statValue, _ := strconv.Atoi(statValueStr)
		switch statName {
		case "HP":
			pokemon.HP = statValue
		case "ATTACK":
			pokemon.Attack = statValue
		case "DEFENSE":
			pokemon.Defense = statValue
		case "SP ATK":
			pokemon.SpAttack = statValue
		case "SP DEF":
			pokemon.SpDefense = statValue
		case "SPEED":
			pokemon.Speed = statValue
		}
	})

	pokemon.Description = strings.TrimSpace(doc.Find(".monster-description").Text())
	pokemon.Height = strings.TrimSpace(doc.Find(".monster-minutia span").Eq(0).Text())
	pokemon.Weight = strings.TrimSpace(doc.Find(".monster-minutia span").Eq(1).Text())

	pokemon.TotalEVs = pokemon.HP + pokemon.Attack + pokemon.Defense + pokemon.SpAttack + pokemon.SpDefense + pokemon.Speed

	return pokemon, nil
}

func fetchExpAndImage(index string) (int, string, string, error) {
	url := "https://bulbapedia.bulbagarden.net/wiki/List_of_Pok%C3%A9mon_by_effort_value_yield_(Generation_IX)"
	resp, err := http.Get(url)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to fetch Bulbapedia page: %v", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to parse Bulbapedia page HTML: %v", err)
	}

	pokemonIndex := fmt.Sprintf("%04s", index)
	var expText, imageURL, name string
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		if strings.TrimSpace(s.Find("td").First().Text()) == pokemonIndex {
			expText = s.Find("td").Eq(3).Text()
			imageURL = s.Find("td").Eq(1).Find("img").AttrOr("src", "")
			name = s.Find("td").Eq(2).Text()
			return
		}
	})

	expText = strings.TrimSpace(expText)
	exp, err := strconv.Atoi(expText)
	if err != nil {
		return 0, "", "", fmt.Errorf("failed to parse EXP value: %v", err)
	}

	return exp, imageURL, name, nil
}

func fetchPokemons(ctx context.Context) ([]Pokemon, error) {
	html, err := fetchMainPageHTML(ctx)
	if err != nil {
		return nil, err
	}

	names, urls, err := parseMainPage(html)
	fmt.Printf("Found %d Pokémon\n", len(names))
	fmt.Printf("Found %d Pokémon\n", len(urls))
	if err != nil {
		return nil, err
	}

	var pokemons []Pokemon
	for i, url := range urls {
		name := names[i]
		fmt.Printf("Fetching data for %s (%s)\n", name, url)
		pokemonHTML, err := fetchPokemonPageHTML(ctx, url)
		if err != nil {
			fmt.Printf("Error fetching data for %s: %v\n", name, err)
			continue
		}
		pokemon, err := parsePokemonPage(pokemonHTML, name, strconv.Itoa(i+1))
		if err != nil {
			fmt.Printf("Error parsing data for %s: %v\n", name, err)
			continue
		}
		exp, imageURL, newName, err := fetchExpAndImage(pokemon.Index)
		if err != nil {
			fmt.Printf("Error fetching EXP and image for %s: %v\n", pokemon.Name, err)
		}
		// if pokemon.Name == "" && imageURL != "https://archives.bulbagarden.net/media/upload/thumb/0/02/0250Ho-Oh.png/250px-0250Ho-Oh.png" {
		// 	var partOfImageURL = strings.Split(strings.Split(imageURL, "-")[1], ".")
		// 	fmt.Println(partOfImageURL)
		// 	// elminates the 4 fist element of the slice
		// 	pokemon.Name = partOfImageURL[0][4:]
		// }
		if pokemon.Name == "" {
			pokemon.Name = newName[:len(newName)-1]
		}
		pokemon.Exp = exp
		pokemon.ImageURL = imageURL
		pokemons = append(pokemons, pokemon)
		fmt.Printf("Fetched: %+v\n", pokemon)
	}

	return pokemons, nil
}

func main() {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
	}
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()

	pokemons, err := fetchPokemons(ctx)
	if err != nil {
		log.Fatalf("Error fetching Pokémon data: %v", err)
	}

	file, err := os.Create("pokedex.json")
	if err != nil {
		log.Fatalf("Error creating file: %v", err)
	}
	defer file.Close()

	jsonData, err := json.MarshalIndent(pokemons, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling JSON: %v", err)
	}
	file.Write(jsonData)

	fmt.Println("Pokedex data has been written to pokedex.json")
}
