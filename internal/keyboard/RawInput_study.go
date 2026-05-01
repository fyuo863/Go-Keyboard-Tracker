package keyboard

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func KeyboardStart() {
	fmt.Println("正在启动键盘监听器...")

	// 1. 调用底层黑盒，获取数据通道
	eventChan, err := StartWindowsInputListener()
	if err != nil {
		fmt.Println("启动失败:", err)
		return
	}
	fmt.Println("启动成功！请在后台敲击键盘...")

	// 你可以自己设计一个结构体来保存统计数据（想想该怎么设计？）
	// myStats := make(map[uint16]int)

	// 2. 启动一个你自己的协程，专门处理业务逻辑
	go func() {
		// 不断从通道中接收底层的键盘事件
		for event := range eventChan {

			// 过滤掉按键抬起事件，只统计“按下”
			if !event.IsDown {
				continue
			}

			// --- 在这里写你的业务逻辑 ---
			// 比如：myStats[event.VKCode]++
			// 比如：如果是某个特殊的键，触发个什么功能

			fmt.Printf("[业务层收到数据] 按键码: 0x%02X \n", event.VKCode)
		}
	}()

	// 3. 这里你可以尝试再写一个协程，比如定时任务 (每 10 秒打印一次你的统计结果？或者保存到 json 文件里？)
	go func() {
		for {
			time.Sleep(10 * time.Second)
			// TODO: 打印或保存你的数据
			// fmt.Println("10秒过去了，当前统计数据是...")
		}
	}()

	// 4. 主协程阻塞，等待优雅退出 (Ctrl+C)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n主程序准备退出，你可以在这里执行最后的数据保存逻辑！")
	// TODO: 最后保存一次文件
}
