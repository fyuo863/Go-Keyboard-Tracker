package keyboard

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// KeyEvent 定义抛给你的纯净 Go 结构体
type KeyEvent struct {
	VKCode uint16 // 虚拟键码 (比如 0x41 是 'A')
	IsDown bool   // true 表示按下，false 表示抬起
}

// 供底层回调函数使用的全局通道
var keyEventChan chan KeyEvent

// StartWindowsInputListener 是供你调用的核心黑盒函数
// 它会返回一个通道，你可以从这个通道里源源不断地读取键盘事件
func StartWindowsInputListener() (<-chan KeyEvent, error) {
	// 设置一个容量为 1000 的缓冲通道，防止你的 Go 代码处理太慢导致 Windows 底层阻塞
	keyEventChan = make(chan KeyEvent, 1000)

	readyChan := make(chan struct{})

	// 启动底层工作线程
	go startRawInputLoop(readyChan)

	// 等待底层初始化完成
	<-readyChan

	return keyEventChan, nil
}

// ==========================================
// 下面全都是底层 Windows API 的实现，你不需要关心细节
// ==========================================

const (
	WM_INPUT                   = 0x00FF
	RID_INPUT                  = 0x10000003
	RIM_TYPEKEYBOARD           = 1
	RIDEV_INPUTSINK            = 0x00000100
	HID_USAGE_PAGE_GENERIC     = 0x01
	HID_USAGE_GENERIC_KEYBOARD = 0x06
	RI_KEY_BREAK               = 0x0001
)

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

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procRegisterRawInputDev = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData     = user32.NewProc("GetRawInputData")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
)

func wndProc(hwnd windows.Handle, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	if msg == WM_INPUT {
		var dwSize uint32
		headerSize := uint32(unsafe.Sizeof(RAWINPUTHEADER{}))
		procGetRawInputData.Call(lParam, RID_INPUT, 0, uintptr(unsafe.Pointer(&dwSize)), uintptr(headerSize))

		if dwSize > 0 {
			buffer := make([]byte, dwSize)
			procGetRawInputData.Call(lParam, RID_INPUT, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&dwSize)), uintptr(headerSize))

			header := (*RAWINPUTHEADER)(unsafe.Pointer(&buffer[0]))
			if header.DwType == RIM_TYPEKEYBOARD {
				keyboardData := (*RAWKEYBOARD)(unsafe.Pointer(&buffer[unsafe.Sizeof(RAWINPUTHEADER{})]))

				// 构造干净的 Go 结构体
				event := KeyEvent{
					VKCode: keyboardData.VKey,
					IsDown: keyboardData.Flags&RI_KEY_BREAK == 0,
				}

				// 使用 select 非阻塞发送，防止通道满了卡死底层回调
				select {
				case keyEventChan <- event:
				default:
					fmt.Println("警告：处理事件过慢，部分按键被丢弃")
				}
			}
		}
	}
	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func startRawInputLoop(ready chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInst, _, _ := procGetModuleHandle.Call(0)
	hInstance := windows.Handle(hInst)
	className, _ := windows.UTF16PtrFromString("RawInputHiddenClass")

	wcex := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   windows.NewCallback(wndProc),
		HInstance:     hInstance,
		LpszClassName: className,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcex)))

	const HWND_MESSAGE = ^uintptr(2)
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), 0, 0,
		0, 0, 0, 0, HWND_MESSAGE, 0, uintptr(hInstance), 0,
	)

	rid := RAWINPUTDEVICE{
		UsUsagePage: HID_USAGE_PAGE_GENERIC,
		UsUsage:     HID_USAGE_GENERIC_KEYBOARD,
		DwFlags:     RIDEV_INPUTSINK,
		HwndTarget:  windows.Handle(hwnd),
	}
	procRegisterRawInputDev.Call(uintptr(unsafe.Pointer(&rid)), 1, uintptr(unsafe.Sizeof(rid)))

	close(ready)

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
