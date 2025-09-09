package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"
)

// Habit represents a task or goal you want to achieve.
type Habit struct {
	Name        string `json:"name"`
	XPValue     int    `json:"xpValue"`
	IsCompleted bool   `json:"isCompleted"`
}

// Player represents your character in this leveling system.
type Player struct {
	Level        int     `json:"level"`
	XP           int     `json:"xp"`
	XPToNextLevel int     `json:"xpToNextLevel"`
	Strength     int     `json:"strength"`     // Represents physical habits (exercise, etc.)
	Intelligence int     `json:"intelligence"` // Represents mental habits (studying, reading, etc.)
	Dexterity    int     `json:"dexterity"`    // Represents skill-based habits (practice, coding, etc.)
	Habits       []Habit `json:"habits"`
}

// The global player state.
var player Player

// Initializes the player with default values.
func initializePlayer() {
	player = Player{
		Level:        1,
		XP:           0,
		XPToNextLevel: 100,
		Strength:     1,
		Intelligence: 1,
		Dexterity:    1,
		Habits:       []Habit{},
	}
}

// levelUp handles the logic for when the player gains a new level.
func (p *Player) levelUp() {
	for p.XP >= p.XPToNextLevel {
		p.Level++
		p.XP -= p.XPToNextLevel
		// Increase the XP requirement for the next level (e.g., by 50%)
		p.XPToNextLevel = int(float64(p.XPToNextLevel) * 1.5)

		// Grant stat points on level up
		p.Strength++
		p.Intelligence++
		p.Dexterity++
	}
}

// addHabit creates a new habit and adds it to the player's list.
// It's exposed to JavaScript.
func addHabit(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeString {
		fmt.Println("Error: Habit name not provided or not a string.")
		return nil
	}
	habitName := args[0].String()

	newHabit := Habit{
		Name:        habitName,
		XPValue:     25, // All habits give 25 XP for now
		IsCompleted: false,
	}
	player.Habits = append(player.Habits, newHabit)

	fmt.Printf("Added new habit: %s\n", habitName)
	return getPlayerData(js.Value{}, []js.Value{})
}

// completeHabit marks a habit as complete, awards XP, and checks for level-ups.
// It's exposed to JavaScript.
func completeHabit(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 || args[0].Type() != js.TypeNumber {
		fmt.Println("Error: Habit index not provided or not a number.")
		return nil
	}
	index := args[0].Int()

	if index < 0 || index >= len(player.Habits) {
		fmt.Printf("Error: Invalid habit index %d\n", index)
		return nil
	}

	habit := &player.Habits[index]
	
	// For simplicity, we remove the habit on completion.
	// You could also mark it as completed and reset daily.
	player.XP += habit.XPValue
	fmt.Printf("Completed habit '%s' for %d XP.\n", habit.Name, habit.XPValue)
	
	// Remove the completed habit from the slice
	player.Habits = append(player.Habits[:index], player.Habits[index+1:]...)

	player.levelUp()

	return getPlayerData(js.Value{}, []js.Value{})
}

// getPlayerData serializes the current player state to JSON and returns it.
// It's exposed to JavaScript.
func getPlayerData(this js.Value, args []js.Value) interface{} {
	data, err := json.Marshal(player)
	if err != nil {
		fmt.Println("Error marshalling player data:", err)
		return ""
	}
	return string(data)
}

// loadPlayerData deserializes a JSON string to update the player state.
// Used for loading data from the browser's local storage.
func loadPlayerData(this js.Value, args []js.Value) interface{} {
    if len(args) == 0 || args[0].Type() != js.TypeString {
		fmt.Println("Error: Saved data not provided or not a string.")
		// If load fails, return the initial state
		initializePlayer()
		return getPlayerData(js.Value{}, []js.Value{})
	}
	jsonString := args[0].String()
	err := json.Unmarshal([]byte(jsonString), &player)
	if err != nil {
		fmt.Println("Error unmarshalling saved data:", err)
        // If there's an error, re-initialize to a safe state.
		initializePlayer()
	} else {
        fmt.Println("Player data loaded successfully.")
    }
	return getPlayerData(js.Value{}, []js.Value{})
}


// registerCallbacks sets up the Go functions that JavaScript can call.
func registerCallbacks() {
	js.Global().Set("addHabit", js.FuncOf(addHabit))
	js.Global().Set("completeHabit", js.FuncOf(completeHabit))
	js.Global().Set("getPlayerData", js.FuncOf(getPlayerData))
	js.Global().Set("loadPlayerData", js.FuncOf(loadPlayerData))
}

func main() {
	// A channel to keep the Go program running.
	c := make(chan struct{}, 0)

	fmt.Println("Go WebAssembly Solo Leveling System Initialized")

	// Initialize the player state when the program starts.
	initializePlayer()
	
	// Register the functions that JavaScript will call.
	registerCallbacks()

	// Keep the program alive.
	<-c
}
