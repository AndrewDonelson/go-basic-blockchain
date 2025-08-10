package menu

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

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
	clearScreen()

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
	// Stop progress indicator when menu opens
	if ms.ProgressIndicator != nil {
		ms.ProgressIndicator.Stop()
	}

	// Set menu active state in blockchain
	if ms.Blockchain != nil {
		ms.Blockchain.SetMenuActive(true)
	}

	// Clear screen and take over terminal
	clearScreen()
	fmt.Println("Menu system active - all blockchain output paused")

	ms.IsActive = true
	defer func() {
		ms.IsActive = false

		// Set menu inactive state in blockchain
		if ms.Blockchain != nil {
			ms.Blockchain.SetMenuActive(false)
		}

		// Start progress indicator when menu closes
		if ms.ProgressIndicator != nil {
			ms.ProgressIndicator.Start()
		}

		// Clear screen when exiting menu
		clearScreen()
		fmt.Println("Menu closed - blockchain output resumed")
	}()

	for {
		// Display current menu
		ms.CurrentMenu.Display()

		// Read input
		key := ms.readKey()

		// Skip empty input (timeout)
		if key == "" {
			// Small delay to prevent CPU spinning
			time.Sleep(50 * time.Millisecond)
			continue
		}

		switch key {
		case "up":
			ms.CurrentMenu.CurrentItem--
			if ms.CurrentMenu.CurrentItem < 0 {
				ms.CurrentMenu.CurrentItem = len(ms.CurrentMenu.Items) - 1
			}
		case "down":
			ms.CurrentMenu.CurrentItem++
			if ms.CurrentMenu.CurrentItem >= len(ms.CurrentMenu.Items) {
				ms.CurrentMenu.CurrentItem = 0
			}
		case "enter":
			if len(ms.CurrentMenu.Items) > 0 {
				selectedItem := ms.CurrentMenu.Items[ms.CurrentMenu.CurrentItem]

				if selectedItem.SubMenu != nil {
					// Navigate to submenu
					ms.CurrentMenu = selectedItem.SubMenu
				} else if selectedItem.Action != nil {
					// Execute action
					clearScreen()
					fmt.Printf("Executing: %s\n\n", selectedItem.Title)

					if err := selectedItem.Action(); err != nil {
						fmt.Printf("Error: %v\n", err)
					}

					fmt.Printf("\nPress any key to continue...")
					ms.readKey()
				}
			}
		case "escape":
			if ms.CurrentMenu.Parent != nil {
				// Go back to parent menu
				ms.CurrentMenu = ms.CurrentMenu.Parent
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

// readKey reads a single key press with timeout to prevent blocking
func (ms *MenuSystem) readKey() string {
	// Use a channel to handle input with timeout
	inputChan := make(chan string, 1)

	go func() {
		reader := bufio.NewReader(os.Stdin)

		// Read first byte
		char, _, err := reader.ReadRune()
		if err != nil {
			inputChan <- ""
			return
		}

		// Check for escape sequence
		if char == 27 {
			// Read next character
			next, _, err := reader.ReadRune()
			if err != nil {
				inputChan <- "escape"
				return
			}

			if next == 91 {
				// Read the third character
				third, _, err := reader.ReadRune()
				if err != nil {
					inputChan <- "escape"
					return
				}

				switch third {
				case 65:
					inputChan <- "up"
					return
				case 66:
					inputChan <- "down"
					return
				case 67:
					inputChan <- "right"
					return
				case 68:
					inputChan <- "left"
					return
				}
			}

			inputChan <- "escape"
			return
		}

		// Check for special keys
		switch char {
		case 13:
			inputChan <- "enter"
			return
		case 113, 81: // 'q' or 'Q'
			inputChan <- "q"
			return
		}

		inputChan <- string(char)
	}()

	// Wait for input with a short timeout
	select {
	case key := <-inputChan:
		return key
	case <-time.After(100 * time.Millisecond):
		return "" // No input received
	}
}

// clearScreen clears the terminal screen
func clearScreen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// centerText centers text within a given width
func centerText(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}

	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text
}
