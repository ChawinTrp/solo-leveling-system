package main

import (
	"encoding/json" // Keep for string formatting
	"math"
	"syscall/js" // Required for JS interop
	"time"

	"github.com/google/uuid"
)

// --- JS Logging Helper ---
// jsLog sends a log message to the browser's console.
func jsLog(args ...interface{}) {
	js.Global().Get("console").Call("log", args...)
}

// --- Data Models ---

type Class struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Color         string `json:"color"`
	Level         int    `json:"level"`
	XP            int    `json:"xp"`
	XPToNextLevel int    `json:"xpToNextLevel"`
}

type Quest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	ClassID string `json:"classId"`
	XP      int    `json:"xp"` // Measured in hours
}

type HistoryEntry struct {
	QuestName   string    `json:"questName"`
	ClassID     string    `json:"classId"`
	XP          int       `json:"xp"`
	CompletedAt time.Time `json:"completedAt"`
}

type Project struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	CreatedAt    time.Time        `json:"createdAt"`
	LastActivity time.Time        `json:"lastActivity"`
	Quests       map[string]Quest `json:"quests"` // map[Quest.ID]Quest
	History      []HistoryEntry   `json:"history"`
	TotalXP      int              `json:"totalXp"`
	IsArchived   bool             `json:"isArchived"`
}

type Player struct {
	Classes  map[string]Class   `json:"classes"`  // map[Class.ID]Class
	Projects map[string]Project `json:"projects"` // map[Project.ID]Project
}

// --- Leveling Logic ---

// totalXPToReachLevel calculates the *total* XP needed to reach a given level from level 1.
// Formula: TotalXP(L) = 10 * (L - 1)^1.5
// We use float64 for precision in the calculation.
func totalXPToReachLevel(level int) float64 {
	if level <= 1 {
		return 0
	}
	// Total XP to reach level L
	return 10.0 * math.Pow(float64(level-1), 1.5)
}

// calculateXPForLevel calculates the XP required to get from `level` to `level+1`.
// This is the difference between the total XP for the next level and the total XP for the current level.
func calculateXPForLevel(level int) int {
	if level < 1 {
		level = 1
	}
	xpForCurrentLevel := totalXPToReachLevel(level)
	xpForNextLevel := totalXPToReachLevel(level + 1)

	// We round up to ensure a clean integer requirement
	return int(math.Ceil(xpForNextLevel - xpForCurrentLevel))
}

// internal_checkClassLevelUp handles leveling up a class.
// It's a loop to handle multi-level-ups from a single large quest.
func internal_checkClassLevelUp(class *Class) {
	// Loop as long as the current XP is enough to level up
	for class.XP >= class.XPToNextLevel {
		// Level up!
		class.Level++
		// Subtract the XP requirement for the *previous* level
		class.XP -= class.XPToNextLevel
		// Calculate the *new* XP requirement for the *new* level
		class.XPToNextLevel = calculateXPForLevel(class.Level)
	}
}

// --- Global State ---
// This holds the entire application state in memory.
var player Player

// --- Exposed Go Functions ---

// getPlayerData initializes a new player state.
func getPlayerData(this js.Value, args []js.Value) interface{} {
	jsLog("go_getPlayerData called")
	player = Player{
		Classes:  make(map[string]Class),
		Projects: make(map[string]Project),
	}
	return mustMarshal(player)
}

// loadPlayerData loads the player state from a JSON string (from Firebase).
func loadPlayerData(this js.Value, args []js.Value) interface{} {
	jsLog("go_loadPlayerData called")
	jsonString := args[0].String()

	// Default to new player state
	player = Player{
		Classes:  make(map[string]Class),
		Projects: make(map[string]Project),
	}

	if jsonString == "" || jsonString == "null" || jsonString == "{}" {
		jsLog("Empty or null data received, returning new player state.")
		return mustMarshal(player)
	}

	var loadedPlayer Player
	if err := json.Unmarshal([]byte(jsonString), &loadedPlayer); err != nil {
		jsLog("Error unmarshaling player data:", err.Error())
		// Return a new player state on error
		return mustMarshal(player)
	}

	// Ensure maps and slices are not nil (critical from JSON)
	if loadedPlayer.Classes == nil {
		loadedPlayer.Classes = make(map[string]Class)
	}
	if loadedPlayer.Projects == nil {
		loadedPlayer.Projects = make(map[string]Project)
	}

	// Ensure nested maps/slices are not nil for *each* project
	for id, proj := range loadedPlayer.Projects {
		if proj.Quests == nil {
			proj.Quests = make(map[string]Quest)
		}
		if proj.History == nil {
			proj.History = []HistoryEntry{}
		}
		// Write the corrected project back to the map
		loadedPlayer.Projects[id] = proj
	}

	player = loadedPlayer
	return mustMarshal(player)
}

// createClass creates a new skill class.
func createClass(this js.Value, args []js.Value) interface{} {
	name := args[0].String()
	color := args[1].String()
	id := uuid.NewString()
	jsLog("go_createClass called:", name, color)

	newClass := Class{
		ID:            id,
		Name:          name,
		Color:         color,
		Level:         1,
		XP:            0,
		XPToNextLevel: calculateXPForLevel(1), // XP to get from Lvl 1 to Lvl 2
	}

	if player.Classes == nil {
		player.Classes = make(map[string]Class)
	}
	player.Classes[id] = newClass
	return mustMarshal(player)
}

// createProject creates a new project container.
func createProject(this js.Value, args []js.Value) interface{} {
	name := args[0].String()
	id := uuid.NewString()
	now := time.Now()
	jsLog("go_createProject called:", name)

	newProject := Project{
		ID:           id,
		Name:         name,
		CreatedAt:    now,
		LastActivity: now,
		Quests:       make(map[string]Quest),
		History:      []HistoryEntry{},
		TotalXP:      0,
		IsArchived:   false,
	}

	if player.Projects == nil {
		player.Projects = make(map[string]Project)
	}
	player.Projects[id] = newProject
	return mustMarshal(player)
}

// addQuestToProject adds a new quest to a specific project.
func addQuestToProject(this js.Value, args []js.Value) interface{} {
	projectID := args[0].String()
	classID := args[1].String()
	questName := args[2].String()
	hours := args[3].Int() // JS will pass this as a number
	jsLog("go_addQuestToProject called:", projectID, questName)

	project, ok := player.Projects[projectID]
	if !ok {
		jsLog("Project not found:", projectID)
		return mustMarshal(player)
	}

	// Check if class exists
	if _, ok := player.Classes[classID]; !ok {
		jsLog("Class not found:", classID)
		return mustMarshal(player)
	}

	if hours <= 0 {
		jsLog("Quest hours must be positive:", hours)
		return mustMarshal(player)
	}

	newQuest := Quest{
		ID:      uuid.NewString(),
		Name:    questName,
		ClassID: classID,
		XP:      hours,
	}

	if project.Quests == nil {
		project.Quests = make(map[string]Quest)
	}

	project.Quests[newQuest.ID] = newQuest
	project.LastActivity = time.Now()
	player.Projects[projectID] = project // Write back the modified struct

	return mustMarshal(player)
}

// completeQuest completes a quest, moves it to history, and applies XP.
func completeQuest(this js.Value, args []js.Value) interface{} {
	projectID := args[0].String()
	questID := args[1].String()
	jsLog("go_completeQuest called:", projectID, questID)

	// Find Project
	project, ok := player.Projects[projectID]
	if !ok {
		jsLog("Project not found:", projectID)
		return mustMarshal(player)
	}

	// Find Quest in Project
	quest, ok := project.Quests[questID]
	if !ok {
		jsLog("Quest not found:", questID, "in project:", projectID)
		return mustMarshal(player)
	}

	// 1. Create History Entry
	historyEntry := HistoryEntry{
		QuestName:   quest.Name,
		ClassID:     quest.ClassID,
		XP:          quest.XP,
		CompletedAt: time.Now(),
	}
	if project.History == nil {
		project.History = []HistoryEntry{}
	}
	project.History = append(project.History, historyEntry)

	// 2. Apply XP to Project
	project.TotalXP += quest.XP

	// 3. Remove from active quests
	delete(project.Quests, questID)

	// 4. Update project activity
	project.LastActivity = time.Now()
	player.Projects[projectID] = project // Write back modified project

	// 5. Apply XP to Class and check for level up
	class, ok := player.Classes[quest.ClassID]
	if ok {
		class.XP += quest.XP
		internal_checkClassLevelUp(&class)    // Pass by reference to modify
		player.Classes[quest.ClassID] = class // Write back modified class
	} else {
		jsLog("Warning: Class not found for completed quest:", quest.ClassID)
	}

	return mustMarshal(player)
}

// archiveInactiveProjects archives projects with no activity in 30 days.
func archiveInactiveProjects(this js.Value, args []js.Value) interface{} {
	jsLog("go_archiveInactiveProjects called")
	now := time.Now()
	// 30 days * 24 hours/day
	thirtyDays := time.Hour * 24 * 30

	for id, project := range player.Projects {
		// Only check non-archived projects
		if !project.IsArchived {
			if now.Sub(project.LastActivity) > thirtyDays {
				project.IsArchived = true
				player.Projects[id] = project
			}
		}
	}
	return mustMarshal(player)
}

// --- Utils ---

// mustMarshal marshals the player state to JSON and returns a string.
// Panics on error because if we can't marshal our own state, something is wrong.
func mustMarshal(p Player) string {
	data, err := json.Marshal(p)
	if err != nil {
		// Don't panic, just log and return an empty state
		jsLog("FATAL: Failed to marshal player state:", err.Error())
		return `{"classes":{}, "projects":{}}`
	}
	return string(data)
}

// registerCallbacks registers all Go functions to be callable from JS.
func registerCallbacks() {
	js.Global().Set("go_getPlayerData", js.FuncOf(getPlayerData))
	js.Global().Set("go_loadPlayerData", js.FuncOf(loadPlayerData))
	js.Global().Set("go_createClass", js.FuncOf(createClass))
	js.Global().Set("go_createProject", js.FuncOf(createProject))
	js.Global().Set("go_addQuestToProject", js.FuncOf(addQuestToProject))
	js.Global().Set("go_completeQuest", js.FuncOf(completeQuest))
	js.Global().Set("go_archiveInactiveProjects", js.FuncOf(archiveInactiveProjects))

	// *** NEW: Signal to JS that all functions are registered and Go is ready ***
	jsLog("Go (WASM) callbacks registered. Signaling ready.")
	js.Global().Call("goWasmReady")
}

// --- Main ---

func main() {
	// *** MODIFIED: Updated log message ***
	jsLog("Go (WASM) Aura Engine main function started.")
	registerCallbacks()
	// Keep the program alive forever
	select {}
}
