package main

import (
	"encoding/json"
	"fmt"
	"math"
	"syscall/js"
	"time"
)

// HistoryEntry stores information about a completed habit.
type HistoryEntry struct {
	Name        string `json:"name"`
	CompletedAt string `json:"completedAt"`
}

// Habit represents an active quest or task.
type Habit struct {
	Name    string `json:"name"`
	XPValue int    `json:"xpValue"`
}

// Player holds all the character's data, including the new history log.
type Player struct {
	Level         int            `json:"level"`
	XP            int            `json:"xp"`
	XPToNextLevel int            `json:"xpToNextLevel"`
	Strength      int            `json:"strength"`
	Intelligence  int            `json:"intelligence"`
	Dexterity     int            `json:"dexterity"`
	Habits        []Habit        `json:"habits"`
	History       []HistoryEntry `json:"history"` // New field for the log
}

var player Player

func main() {
	fmt.Println("Go WebAssembly Initialized")

	// Initialize player with default values
	player = Player{
		Level:         1,
		XP:            0,
		XPToNextLevel: 100,
		Strength:      1,
		Intelligence:  1,
		Dexterity:     1,
		Habits:        []Habit{},
		History:       []HistoryEntry{}, // Initialize the history slice
	}

	registerCallbacks()
	<-make(chan bool) // Keep the program alive
}

// getPlayerData converts the player struct to a JSON string.
func getPlayerData(this js.Value, args []js.Value) interface{} {
	jsonData, err := json.Marshal(player)
	if err != nil {
		fmt.Println("Error marshalling player data:", err)
		return ""
	}
	return string(jsonData)
}

// addHabit adds a new habit to the player's list.
func addHabit(this js.Value, args []js.Value) interface{} {
	habitName := args[0].String()
	newHabit := Habit{Name: habitName, XPValue: 25} // You can customize XP here
	player.Habits = append(player.Habits, newHabit)
	return getPlayerData(this, args)
}

// completeHabit awards XP, moves the habit to history, and checks for level up.
func completeHabit(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeNumber {
		fmt.Println("Invalid index for completeHabit")
		return getPlayerData(this, args)
	}
	index := args[0].Int()

	if index < 0 || index >= len(player.Habits) {
		fmt.Println("Index out of bounds for habits")
		return getPlayerData(this, args)
	}

	// Get the habit to be completed
	habit := player.Habits[index]

	// Award XP
	player.XP += habit.XPValue

	// Add to history with a timestamp
	historyEntry := HistoryEntry{
		Name:        habit.Name,
		CompletedAt: time.Now().Format("02 Jan 2006 15:04"), // User-friendly timestamp
	}
	// --- DEBUGGING BLOCK START ---
	fmt.Printf("[Go DEBUG] History BEFORE append: %d items\n", len(player.History))
	fmt.Printf("[Go DEBUG] Adding entry to history: %+v\n", historyEntry) // %+v prints struct with field names

	// This is the line we are testing
	player.History = append([]HistoryEntry{historyEntry}, player.History...)

	fmt.Printf("[Go DEBUG] History AFTER append: %d items\n", len(player.History))
	if len(player.History) > 0 {
		fmt.Printf("[Go DEBUG] Newest history entry is now: %+v\n", player.History[0])
	}
	// --- DEBUGGING BLOCK END ---

	// Remove habit from the active list
	player.Habits = append(player.Habits[:index], player.Habits[index+1:]...)

	// Check for level up
	if player.XP >= player.XPToNextLevel {
		levelUp()
	}

	return getPlayerData(this, args)
}

// levelUp handles the logic for increasing player level and stats.
func levelUp() {
	player.Level++
	player.XP -= player.XPToNextLevel // Carry over extra XP
	// Increase XP requirement for the next level
	player.XPToNextLevel = int(100 * math.Pow(float64(player.Level), 1.5))
	// Award stat points
	player.Strength++
	player.Intelligence++
	player.Dexterity++
}

// loadPlayerData populates the player struct from a saved JSON string.
func loadPlayerData(this js.Value, args []js.Value) interface{} {
	savedData := args[0].String()
	err := json.Unmarshal([]byte(savedData), &player)
	if err != nil {
		fmt.Println("Error unmarshalling saved data:", err)
	}
	// Ensure history is not null if loading from older data
	if player.History == nil {
		player.History = []HistoryEntry{}
	}
	return getPlayerData(this, args)
}

// registerCallbacks exposes Go functions to JavaScript.
func registerCallbacks() {
	js.Global().Set("getPlayerData", js.FuncOf(getPlayerData))
	js.Global().Set("addHabit", js.FuncOf(addHabit))
	js.Global().Set("completeHabit", js.FuncOf(completeHabit))
	js.Global().Set("loadPlayerData", js.FuncOf(loadPlayerData))
}
