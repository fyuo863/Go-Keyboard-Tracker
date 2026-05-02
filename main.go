package main

import (
	"fmt"
	"keybord_btop/internal/display"
	"keybord_btop/internal/keyboard"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	recentQueueLimit = 12
	curveBucketLimit = 120
	fastRefreshTick  = 150 * time.Millisecond
	curveRefreshTick = 1 * time.Second
)

type AppState struct {
	mu                 sync.RWMutex
	Stats              map[uint16]int
	RecentKeys         []string
	CurveBuckets       []int
	CurrentBucketCount int
}

func main() {
	app := tview.NewApplication()
	ui := display.BuildDashboard()

	eventChan, err := keyboard.StartWindowsInputListener()
	if err != nil {
		panic(err)
	}

	state := &AppState{Stats: make(map[uint16]int)}

	go func() {
		for ev := range eventChan {
			if !ev.IsDown {
				continue
			}
			state.mu.Lock()
			state.Stats[ev.VKCode]++
			state.CurrentBucketCount++
			state.RecentKeys = append(state.RecentKeys, formatVKCode(ev.VKCode))
			if len(state.RecentKeys) > recentQueueLimit {
				state.RecentKeys = append([]string(nil), state.RecentKeys[len(state.RecentKeys)-recentQueueLimit:]...)
			}
			state.mu.Unlock()
		}
	}()

	go func() {
		ticker := time.NewTicker(fastRefreshTick)
		defer ticker.Stop()
		for range ticker.C {
			statsSnapshot, recentSnapshot := snapshotFastState(state)
			app.QueueUpdateDraw(func() {
				display.RefreshTable(ui.StatsTable, statsSnapshot)
				display.RefreshInputQueue(ui.InputQueueView, recentSnapshot)
			})
		}
	}()

	go func() {
		ticker := time.NewTicker(curveRefreshTick)
		defer ticker.Stop()
		for range ticker.C {
			curveSnapshot := rollCurveBucket(state)
			app.QueueUpdateDraw(func() {
				display.RefreshBrailleGraph(ui.BrailleGraph, curveSnapshot)
			})
		}
	}()

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape:
			app.Stop()
			return nil
		case event.Key() == tcell.KeyLeft:
			ui.PrevTab()
			return nil
		case event.Key() == tcell.KeyRight:
			ui.NextTab()
			return nil
		case event.Rune() == '1':
			ui.SetActiveTab(0)
			return nil
		case event.Rune() == '2':
			ui.SetActiveTab(1)
			return nil
		default:
			return event
		}
	})

	if err := app.SetRoot(ui.Root, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}

func snapshotFastState(state *AppState) (map[uint16]int, []string) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	stats := make(map[uint16]int, len(state.Stats))
	for k, v := range state.Stats {
		stats[k] = v
	}
	recent := append([]string(nil), state.RecentKeys...)
	return stats, recent
}

func rollCurveBucket(state *AppState) []int {
	state.mu.Lock()
	defer state.mu.Unlock()

	state.CurveBuckets = append(state.CurveBuckets, state.CurrentBucketCount)
	if len(state.CurveBuckets) > curveBucketLimit {
		state.CurveBuckets = append([]int(nil), state.CurveBuckets[len(state.CurveBuckets)-curveBucketLimit:]...)
	}
	state.CurrentBucketCount = 0
	return append([]int(nil), state.CurveBuckets...)
}

func formatVKCode(vk uint16) string {
	if vk >= '0' && vk <= '9' {
		return string(rune(vk))
	}
	if vk >= 'A' && vk <= 'Z' {
		return string(rune(vk))
	}

	switch vk {
	case 0x20:
		return "Space"
	case 0x08:
		return "Backspace"
	case 0x09:
		return "Tab"
	case 0x0D:
		return "Enter"
	case 0x10:
		return "Shift"
	case 0x11:
		return "Ctrl"
	case 0x12:
		return "Alt"
	case 0x1B:
		return "Esc"
	case 0x25:
		return "←"
	case 0x26:
		return "↑"
	case 0x27:
		return "→"
	case 0x28:
		return "↓"
	case 0x2E:
		return "Delete"
	default:
		return fmt.Sprintf("Key 0x%02X", vk)
	}
}
