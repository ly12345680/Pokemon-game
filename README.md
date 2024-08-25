# PokeCat n PokeBat - Multiplayer Pokémon Game

## Overview

Welcome to **PokeCat n PokeBat**, a text-based multiplayer Pokémon game developed as part of the Net-Centric Programming course at Vietnamese National University HCMC, International University. The game provides practical experience in network application development using Go Lang, focusing on TCP networking, concurrency, and game data persistence.

## Features

- **Multiplayer Battles**: Engage in turn-based Pokémon battles with other players.
- **Pokémon Capturing**: Explore the game world and capture Pokémon.
- **Real-time Communication**: Players can interact with the game server in real-time.
- **Data Persistence**: Player profiles and Pokémon data are stored and retrieved using JSON files.
- **Web Crawler**: A custom crawler to fetch Pokémon data from the web and populate the game’s Pokédex.

## Project Structure

- **Server**: Manages game state, player interactions, and battles.
- **Player Client**: Interface for players to interact with the game.
- **Web Crawler**: Scrapes data to build the game’s Pokédex.
- **JSON Files**: Used for storing player profiles and Pokémon data.

## Architecture

### Server

- **Network Listener**: Listens for incoming connections.
- **Connection Handler**: Manages client connections.
- **Game Logic**: Handles battles, Pokémon selection, and experience calculations.
- **Data Management**: Loads and saves game data.

### Player Client

- **Network Connection**: Connects to the game server.
- **Concurrency Management**: Uses goroutines for handling multiple tasks simultaneously.

### Web Crawler

- **HTML Parsing**: Uses `chromedp` and `goquery` for web scraping.
- **Data Extraction**: Extracts and stores Pokémon data in JSON format.

## Getting Started

### Prerequisites

- Go Lang installed on your machine.
- Basic understanding of terminal/command line operations.

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/your-username/pokemon-game.git
   cd pokemon-game
2. Run the server:
   ```bash
   go run Server.go
3. Start the player client:
   ```bash
   go run player.go
4. Use the web crawler (optional) to fetch Pokémon data:
   ```bash
   go run crawler.go
