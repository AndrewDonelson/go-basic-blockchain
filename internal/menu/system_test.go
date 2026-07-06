package menu

import "testing"

type testProgress struct {
	paused     bool
	pausedCall bool
	resumed    bool
}

func (p *testProgress) Pause()         { p.paused = true; p.pausedCall = true }
func (p *testProgress) Resume()        { p.paused = false; p.resumed = true }
func (p *testProgress) IsPaused() bool { return p.paused }
func (p *testProgress) Stop()          {}
func (p *testProgress) Start()         {}

type testBlockchain struct{ active bool }

func (b *testBlockchain) SetMenuActive(v bool) { b.active = v }

type testKeySource struct {
	keys []string
	i    int
}

func (s *testKeySource) next() string {
	if s.i >= len(s.keys) {
		return "q"
	}
	k := s.keys[s.i]
	s.i++
	return k
}

func TestNewMenuSystem(t *testing.T) {
	ms := NewMenuSystem()
	if ms == nil {
		t.Fatal("expected non-nil menu system")
	}
	if ms.IsActive {
		t.Fatal("expected inactive menu system by default")
	}
}

func TestAddMenuItemAndSubMenu(t *testing.T) {
	root := &Menu{Title: "root"}
	root.AddMenuItem("id", "title", "desc", nil)
	if len(root.Items) != 1 {
		t.Fatalf("expected 1 menu item, got %d", len(root.Items))
	}

	sub := root.AddSubMenu("sub", "Sub", "sub desc")
	if sub == nil {
		t.Fatal("expected sub menu")
	}
	if sub.Parent != root {
		t.Fatal("expected parent link to root")
	}
}

func TestCenterText(t *testing.T) {
	if got := centerText("abc", 7); got == "abc" {
		t.Fatal("expected centered text with padding")
	}
	if got := centerText("toolong", 3); got != "too" {
		t.Fatalf("expected truncation to width, got %q", got)
	}
}

func TestCreateBlockchainMenu(t *testing.T) {
	ms := CreateBlockchainMenu(nil)
	if ms == nil || ms.RootMenu == nil {
		t.Fatal("expected blockchain menu system with root")
	}
	if ms.CurrentMenu != ms.RootMenu {
		t.Fatal("expected current menu to start at root")
	}
	if len(ms.RootMenu.Items) == 0 {
		t.Fatal("expected root menu items")
	}
}

func TestNavigateExecutesActionAndLifecycle(t *testing.T) {
	originalClear := clearScreenFn
	clearScreenFn = func() {}
	defer func() { clearScreenFn = originalClear }()

	progress := &testProgress{}
	chain := &testBlockchain{}
	ms := NewMenuSystem()
	ms.ProgressIndicator = progress
	ms.Blockchain = chain

	actionCalls := 0
	root := &Menu{Title: "root"}
	root.AddMenuItem("run", "Run", "", func() error {
		actionCalls++
		return nil
	})
	ms.RootMenu = root
	ms.CurrentMenu = root

	keys := &testKeySource{keys: []string{"enter", "q", "q"}}
	ms.readKeyOverride = keys.next

	if err := ms.Navigate(); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	if actionCalls != 1 {
		t.Fatalf("expected action called once, got %d", actionCalls)
	}
	if !progress.pausedCall || !progress.resumed {
		t.Fatal("expected progress indicator pause/resume lifecycle")
	}
	if chain.active {
		t.Fatal("expected blockchain menu active reset to false")
	}
	if ms.IsActive {
		t.Fatal("expected menu inactive after navigate returns")
	}
}

func TestNavigateSubmenuEscapeBackToRoot(t *testing.T) {
	originalClear := clearScreenFn
	clearScreenFn = func() {}
	defer func() { clearScreenFn = originalClear }()

	ms := NewMenuSystem()
	root := &Menu{Title: "root"}
	root.AddSubMenu("sub", "Submenu", "")
	ms.RootMenu = root
	ms.CurrentMenu = root

	keys := &testKeySource{keys: []string{"enter", "escape", "q"}}
	ms.readKeyOverride = keys.next

	if err := ms.Navigate(); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}

	if ms.CurrentMenu != root {
		t.Fatal("expected to return to root menu after submenu escape")
	}
}
