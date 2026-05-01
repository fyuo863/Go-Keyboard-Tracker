package main

import (
	"keybord_btop/internal/display"
	"keybord_btop/internal/keyboard"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	// 1. 创建 tview 应用
	app := tview.NewApplication()

	// 2. 构建界面布局（表格 + 底部状态栏）
	root, statsTable := display.BuildDashboard()

	// 3. 启动 Windows 全局键盘监听
	eventChan, err := keyboard.StartWindowsInputListener()
	if err != nil {
		panic(err)
	}

	// 4. 按键统计数据（主 goroutine 写，ticker goroutine 读，无竞态）
	stats := make(map[uint16]int)

	// 5. 后台 goroutine：消费键盘事件，更新 stats
	go func() {
		for ev := range eventChan {
			if ev.IsDown {
				stats[ev.VKCode]++
			}
		}
	}()

	// 6. 后台 goroutine：60fps 定时刷新 UI
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			app.QueueUpdateDraw(func() {
				display.RefreshTable(statsTable, stats)
			})
		}
	}()

	// 7. 全局按键处理（q / Ctrl+C 退出）
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' || event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape {
			app.Stop()
			return nil
		}
		return event
	})

	// 8. 启动事件循环（阻塞直到 app.Stop()）
	if err := app.SetRoot(root, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
