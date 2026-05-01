package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// --- Win32 常量定义 ---
const (
	WM_INPUT                   = 0x00FF
	RID_INPUT                  = 0x10000003
	RIM_TYPEKEYBOARD           = 1
	RIDEV_INPUTSINK            = 0x00000100 // 关键：允许后台接收
	HID_USAGE_PAGE_GENERIC     = 0x01       // 泛型设备
	HID_USAGE_GENERIC_KEYBOARD = 0x06       // 键盘
	RI_KEY_BREAK               = 0x0001     // 键抬起标志
)

// --- Win32 结构体定义 ---
type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type RAWINPUTDEVICE struct {
	UsUsagePage uint16
	UsUsage     uint16
	DwFlags     uint32
	HwndTarget  windows.Handle
}

type RAWINPUTHEADER struct {
	DwType  uint32
	DwSize  uint32
	HDevice windows.Handle
	WParam  uintptr
}

type RAWKEYBOARD struct {
	MakeCode         uint16
	Flags            uint16
	Reserved         uint16
	VKey             uint16
	Message          uint32
	ExtraInformation uint32
}

type MSG struct {
	Hwnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// --- DLL 与 API 加载 ---
var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procRegisterRawInputDev = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData     = user32.NewProc("GetRawInputData")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
)

// 按键统计
var keyCounts = make(map[uint16]int)

// WndProc 窗口消息处理回调
func WndProc(hwnd windows.Handle, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	if msg == WM_INPUT {
		var dwSize uint32

		// 1. 获取数据大小 (第一次调用 GetRawInputData)
		headerSize := uint32(unsafe.Sizeof(RAWINPUTHEADER{}))
		procGetRawInputData.Call(lParam, RID_INPUT, 0, uintptr(unsafe.Pointer(&dwSize)), uintptr(headerSize))

		if dwSize > 0 {
			// 2. 分配内存并获取真实数据 (第二次调用)
			buffer := make([]byte, dwSize)
			procGetRawInputData.Call(lParam, RID_INPUT, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&dwSize)), uintptr(headerSize))

			// 3. 将字节切片转换为 RAWINPUTHEADER
			header := (*RAWINPUTHEADER)(unsafe.Pointer(&buffer[0]))

			// 确保是键盘事件
			if header.DwType == RIM_TYPEKEYBOARD {
				// 将指针向后移动 HEADER 的大小，读取 RAWKEYBOARD 数据
				keyboardData := (*RAWKEYBOARD)(unsafe.Pointer(&buffer[unsafe.Sizeof(RAWINPUTHEADER{})]))

				// Flags 为 0 表示按下 (Key Down)，我们只统计按下，不统计抬起 (RI_KEY_BREAK)
				if keyboardData.Flags&RI_KEY_BREAK == 0 {
					keyCounts[keyboardData.VKey]++
					fmt.Printf("检测到按键按下: VK_CODE [0x%02X], 累计次数: %d\n", keyboardData.VKey, keyCounts[keyboardData.VKey])
				}
			}
		}
	}

	// 其他消息交由系统默认处理
	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func main() {
	fmt.Println("启动 Raw Input 安全键盘监听模式...")

	readyChan := make(chan struct{})

	// 在独立且锁定的线程中运行窗口和消息循环
	go startRawInputLoop(readyChan)

	<-readyChan
	fmt.Println("隐形窗口注册成功！请在任意界面按下键盘进行测试。按 Ctrl+C 退出。")

	// 拦截退出信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n程序优雅退出。")
}

// 核心逻辑必须锁定在物理线程
func startRawInputLoop(ready chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// --- 修正1：获取真实的系统实例句柄 hInstance ---
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandle := kernel32.NewProc("GetModuleHandleW")
	hInst, _, _ := procGetModuleHandle.Call(0)
	hInstance := windows.Handle(hInst)

	className, _ := windows.UTF16PtrFromString("RawInputHiddenClass")

	// 1. 注册隐形窗口类
	wcex := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   windows.NewCallback(WndProc),
		HInstance:     hInstance,
		LpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	// --- 修正2：使用位反转运算生成兼容 64 位的 -3 句柄 ---
	// 在二进制中，^2 (按位取反) 等价于有符号补码的 -3，这样写在32位和64位下都是安全的
	const HWND_MESSAGE = ^uintptr(2)

	// 2. 创建一个 Message-Only 窗口
	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		0,
		0,          // Style
		0, 0, 0, 0, // x, y, width, height
		HWND_MESSAGE, // <--- 修正此处参数
		0,
		uintptr(hInstance),
		0,
	)
	if hwnd == 0 {
		log.Fatalf("创建隐形窗口失败: %v", err)
	}

	// 3. 注册 Raw Input 设备
	rid := RAWINPUTDEVICE{
		UsUsagePage: HID_USAGE_PAGE_GENERIC,
		UsUsage:     HID_USAGE_GENERIC_KEYBOARD,
		DwFlags:     RIDEV_INPUTSINK, // 关键：即便不在前台也能接收
		HwndTarget:  windows.Handle(hwnd),
	}

	ret, _, err := procRegisterRawInputDev.Call(
		uintptr(unsafe.Pointer(&rid)),
		1,
		uintptr(unsafe.Sizeof(rid)),
	)
	if ret == 0 {
		log.Fatalf("注册 Raw Input 失败: %v", err)
	}

	// 通知主协程已就绪
	close(ready)

	// 4. 启动消息循环
	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 || ret == ^uintptr(0) {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
