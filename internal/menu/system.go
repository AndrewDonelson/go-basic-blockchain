package menu

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/term"
)

var clearScreenFn = clearScreen

// MenuItem represents a single menu option
type MenuItem struct {
	ID          string
	Title       string
	Description string
	Action      func() error
	SubMenu     *Menu
}

// Menu represents a menu with items and navigation
type Menu struct {
	Title       string
	Items       []*MenuItem
	CurrentItem int
	Parent      *Menu
	ExitAction  func()
}

// MenuSystem manages the overall menu system
type MenuSystem struct {
	CurrentMenu       *Menu
	RootMenu          *Menu
	IsActive          bool
	readKeyOverride   func() string
	ProgressIndicator interface {
		Pause()
		Resume()
		IsPaused() bool
		Stop()
		Start()
	}
	Blockchain interface {
		SetMenuActive(bool)
	}
}

// NewMenuSystem creates a new menu system
func NewMenuSystem() *MenuSystem {
	return &MenuSystem{
		IsActive: false,
	}
}

// AddMenuItem adds a menu item to the current menu
func (m *Menu) AddMenuItem(id, title, description string, action func() error) {
	item := &MenuItem{
		ID:          id,
		Title:       title,
		Description: description,
		Action:      action,
	}
	m.Items = append(m.Items, item)
}

// AddSubMenu adds a submenu to a menu item
func (m *Menu) AddSubMenu(id, title, description string) *Menu {
	subMenu := &Menu{
		Title:  title,
		Parent: m,
	}

	item := &MenuItem{
		ID:          id,
		Title:       title,
		Description: description,
		SubMenu:     subMenu,
	}
	m.Items = append(m.Items, item)

	return subMenu
}

// Display renders the current menu
func (m *Menu) Display() {
	clearScreenFn()

	// Display menu title
	fmt.Printf("\n╔══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ %-60s ║\n", centerText(m.Title, 60))
	fmt.Printf("╚══════════════════════════════════════════════════════════════╝\n\n")

	// Display menu items
	for i, item := range m.Items {
		if i == m.CurrentItem {
			fmt.Printf("  ▶ %s\n", item.Title)
			if item.Description != "" {
				fmt.Printf("     %s\n", item.Description)
			}
		} else {
			fmt.Printf("    %s\n", item.Title)
		}
	}

	fmt.Printf("\n")
	fmt.Printf("  Use ↑↓ arrows to navigate, ENTER to select, ESC to exit\n")
}

// Navigate handles menu navigation
func (ms *MenuSystem) Navigate() error {
	readKey := ms.readKey
	if ms.readKeyOverride != nil {
		readKey = ms.readKeyOverride
	}

	var previousTermState *term.State
	if ms.readKeyOverride == nil && term.IsTerminal(int(os.Stdin.Fd())) {
		state, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err == nil {
			previousTermState = state
		}
	}

	// Pause progress indicator when menu opens.
	if ms.ProgressIndicator != nil {
		ms.ProgressIndicator.Pause()
	}

	// Set menu active state in blockchain
	if ms.Blockchain != nil {
		ms.Blockchain.SetMenuActive(true)
	}

	// Clear screen and take over terminal
	clearScreenFn()
	fmt.Println("Menu system active - all blockchain output paused")

	ms.IsActive = true
	defer func() {
		if previousTermState != nil {
			//nolint:errcheck // best-effort terminal restore during cleanup
			_ = term.Restore(int(os.Stdin.Fd()), previousTermState)
		}

		ms.IsActive = false

		// Set menu inactive state in blockchain
		if ms.Blockchain != nil {
			ms.Blockchain.SetMenuActive(false)
		}

		// Resume progress indicator when menu closes.
		if ms.ProgressIndicator != nil {
			ms.ProgressIndicator.Resume()
		}

		// Clear screen when exiting menu
		clearScreenFn()
		fmt.Println("Menu closed - blockchain output resumed")
	}()

	needsRender := true
	for {
		if needsRender {
			ms.CurrentMenu.Display()
			needsRender = false
		}

		key := readKey()

		switch key {
		case "up":
			ms.CurrentMenu.CurrentItem--
			if ms.CurrentMenu.CurrentItem < 0 {
				ms.CurrentMenu.CurrentItem = len(ms.CurrentMenu.Items) - 1
			}
			needsRender = true
		case "down":
			ms.CurrentMenu.CurrentItem++
			if ms.CurrentMenu.CurrentItem >= len(ms.CurrentMenu.Items) {
				ms.CurrentMenu.CurrentItem = 0
			}
			needsRender = true
		case "enter":
			if len(ms.CurrentMenu.Items) > 0 {
				selectedItem := ms.CurrentMenu.Items[ms.CurrentMenu.CurrentItem]

				if selectedItem.SubMenu != nil {
					// Navigate to submenu
					ms.CurrentMenu = selectedItem.SubMenu
					needsRender = true
				} else if selectedItem.Action != nil {
					// Execute action
					clearScreenFn()
					fmt.Printf("Executing: %s\n\n", selectedItem.Title)

					if err := selectedItem.Action(); err != nil {
						fmt.Printf("Error: %v\n", err)
					}

					fmt.Printf("\nPress any key to continue...")
					ms.readKey()
					needsRender = true
				}
			}
		case "escape":
			if ms.CurrentMenu.Parent != nil {
				// Go back to parent menu
				ms.CurrentMenu = ms.CurrentMenu.Parent
				needsRender = true
			} else {
				// Exit menu system
				return nil
			}
		case "q":
			// Quick exit
			return nil
		}
	}
}

// readKey reads a single key press.
func (ms *MenuSystem) readKey() string {
	reader := bufio.NewReader(os.Stdin)

	char, _, err := reader.ReadRune()
	if err != nil {
		return ""
	}

	if char == 27 {
		next, _, err := reader.ReadRune()
		if err != nil {
			return "escape"
		}

		if next == 91 {
			third, _, err := reader.ReadRune()
			if err != nil {
				return "escape"
			}

			switch third {
			case 65:
				return "up"
			case 66:
				return "down"
			case 67:
				return "right"
			case 68:
				return "left"
			}
		}

		return "escape"
	}

	switch char {
	case 13, 10:
		return "enter"
	case 113, 81:
		return "q"
	}

	return string(char)
}

// clearScreen clears the terminal screen
func clearScreen() {
	// A bounded context: clearing the screen is cosmetic, and a subprocess that
	// hangs must not hang the menu with it.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", "cls")
	} else {
		cmd = exec.CommandContext(ctx, "clear")
	}
	cmd.Stdout = os.Stdout
	//nolint:errcheck // clearing the screen is cosmetic; a failure changes nothing
	_ = cmd.Run()
}

// centerText centers text within a given width
func centerText(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}

	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text
}
