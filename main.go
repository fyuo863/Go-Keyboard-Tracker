package main

import (
	"keybord_btop/internal/display"
	"keybord_btop/internal/keyboard"
	"os"
	"time"
)

// func main() {
// 	keyboard.KeyboardStart()
// }

func main() {
	// 1. 初始化 UI 可视化器
	viz := display.NewVisualizer()
	oldState, err := viz.Init()
	if err != nil {
		panic(err)
	}
	// 确保退出时恢复终端
	defer viz.Cleanup(oldState)

	// 2. 启动 Windows 底层全局监听 (从我们之前的封装拿)
	eventChan, _ := keyboard.StartWindowsInputListener()

	// 3. 统计数据存储
	stats := make(map[uint16]int)

	// 4. 监听本地键盘输入 (用于按 'q' 退出)
	localKeyChan := make(chan byte)
	go func() {
		buf := make([]byte, 1)
		for {
			os.Stdin.Read(buf)
			localKeyChan <- buf[0]
		}
	}()

	// 5. 渲染计时器 (60 FPS)
	ticker := time.NewTicker(16 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case ev := <-eventChan:
			// 收到全局按键事件
			if ev.IsDown {
				stats[ev.VKCode]++
			}

		case k := <-localKeyChan:
			// 收到本地终端按键 (q 退出)
			if k == 3 { //k == 'q'
				return
			}

		case <-ticker.C:
			// 执行 UI 刷新渲染
			viz.Render(stats)
		}
	}
}
