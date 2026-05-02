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
	recentQueueLimit = 100
	curveBucketLimit = 500
	fastRefreshTick  = 150 * time.Millisecond
	curveRefreshTick = 1 * time.Second
)

type AppState struct {
	mu                 sync.RWMutex
	Stats              map[uint16]int
	RecentKeys         []display.KeyEntry
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
			state.RecentKeys = append(state.RecentKeys, display.KeyEntry{
				Name: formatVKCode(ev.VKCode),
				At:   time.Now(),
			})
			if len(state.RecentKeys) > recentQueueLimit {
				cut := len(state.RecentKeys) - recentQueueLimit
				state.RecentKeys = append([]display.KeyEntry(nil), state.RecentKeys[cut:]...)
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

	// FPS 平滑值，指数移动平均
	var (
		fpsSmooth  float64
		fpsLast    = time.Now()
		fpsAlpha   = 0.05 // 平滑系数，越小越平滑
		fpsCounter int
	)
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		now := time.Now()
		delta := now.Sub(fpsLast).Seconds()
		fpsLast = now
		if delta > 0 && delta < 1.0 {
			instant := 1.0 / delta
			if fpsSmooth == 0 {
				fpsSmooth = instant
			} else {
				fpsSmooth = fpsAlpha*instant + (1-fpsAlpha)*fpsSmooth
			}
		}
		fpsCounter++
		// 每 4 帧更新一次显示，减少绘制开销
		if fpsCounter%4 == 0 {
			fpsText := fmt.Sprintf(" %.0f fps ", fpsSmooth)
			style := tcell.StyleDefault.Foreground(tcell.NewRGBColor(150, 150, 150))
			for i, ch := range fpsText {
				screen.SetContent(i, 0, ch, nil, style)
			}
		}
		return false // false = 正常绘制
	})

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

func snapshotFastState(state *AppState) (map[uint16]int, []display.KeyEntry) {
	state.mu.RLock()
	defer state.mu.RUnlock()

	stats := make(map[uint16]int, len(state.Stats))
	for k, v := range state.Stats {
		stats[k] = v
	}
	recent := make([]display.KeyEntry, len(state.RecentKeys))
	copy(recent, state.RecentKeys)
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
	// —— 鼠标键 ——
	case 0x01:
		return "LBtn"
	case 0x02:
		return "RBtn"
	case 0x04:
		return "MBtn"
	case 0x05:
		return "X1Btn"
	case 0x06:
		return "X2Btn"

	// —— 编辑键 ——
	case 0x08:
		return "⌫"
	case 0x09:
		return "Tab"
	case 0x0D:
		return "Enter"
	case 0x1B:
		return "Esc"
	case 0x20:
		return "␣"
	case 0x2E:
		return "Del"

	// —— 修饰键 ——
	case 0x10:
		return "Shift"
	case 0x11:
		return "Ctrl"
	case 0x12:
		return "Alt"
	case 0x5B:
		return "LWin"
	case 0x5C:
		return "RWin"
	case 0x5D:
		return "Menu"

	// —— 功能键 ——
	case 0x70:
		return "F1"
	case 0x71:
		return "F2"
	case 0x72:
		return "F3"
	case 0x73:
		return "F4"
	case 0x74:
		return "F5"
	case 0x75:
		return "F6"
	case 0x76:
		return "F7"
	case 0x77:
		return "F8"
	case 0x78:
		return "F9"
	case 0x79:
		return "F10"
	case 0x7A:
		return "F11"
	case 0x7B:
		return "F12"

	// —— 方向键 ——
	case 0x25:
		return "←"
	case 0x26:
		return "↑"
	case 0x27:
		return "→"
	case 0x28:
		return "↓"

	// —— 导航键 ——
	case 0x21:
		return "PgUp"
	case 0x22:
		return "PgDn"
	case 0x23:
		return "End"
	case 0x24:
		return "Home"
	case 0x2D:
		return "Ins"

	// —— 锁键 ——
	case 0x14:
		return "CapsLock"
	case 0x90:
		return "NumLock"
	case 0x91:
		return "ScrlLock"

	// —— 数字键盘 ——
	case 0x60:
		return "Num0"
	case 0x61:
		return "Num1"
	case 0x62:
		return "Num2"
	case 0x63:
		return "Num3"
	case 0x64:
		return "Num4"
	case 0x65:
		return "Num5"
	case 0x66:
		return "Num6"
	case 0x67:
		return "Num7"
	case 0x68:
		return "Num8"
	case 0x69:
		return "Num9"
	case 0x6A:
		return "Num*"
	case 0x6B:
		return "Num+"
	case 0x6D:
		return "Num-"
	case 0x6E:
		return "Num."
	case 0x6F:
		return "Num/"

	// —— 符号键 ——
	case 0xBA:
		return ";"
	case 0xBB:
		return "="
	case 0xBC:
		return ","
	case 0xBD:
		return "-"
	case 0xBE:
		return "."
	case 0xBF:
		return "/"
	case 0xC0:
		return "`"
	case 0xDB:
		return "["
	case 0xDC:
		return "\\"
	case 0xDD:
		return "]"
	case 0xDE:
		return "'"

	// —— 媒体键 ——
	case 0xAD:
		return "VolMute"
	case 0xAE:
		return "VolDown"
	case 0xAF:
		return "VolUp"
	case 0xB0:
		return "NextTrk"
	case 0xB1:
		return "PrevTrk"
	case 0xB2:
		return "Stop"
	case 0xB3:
		return "Play"

	// —— 浏览器 / 应用键 ——
	case 0xAC:
		return "Search"
	case 0xA6:
		return "Back"
	case 0xA7:
		return "Forward"

	// —— 截屏 ——
	case 0x2C:
		return "PrtSc"

	// —— 暂停 ——
	case 0x13:
		return "Pause"

	default:
		return fmt.Sprintf("0x%02X", vk)
	}
}
