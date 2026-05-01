package display

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// ═══════════════════════════════════════════════════════════
// ANSI 终端控制码 —— 相当于给终端发送的"指令"，用来清屏、移动光标、变色等
// ═══════════════════════════════════════════════════════════

const (
	Escape       = "\033"            // ESC 字符，所有控制码的开头
	AltScreenOn  = Escape + "[?1049h" // 切换到"备用屏幕"（退出后原终端内容还在）
	AltScreenOff = Escape + "[?1049l" // 切回原屏幕
	HideCursor   = Escape + "[?25l"   // 隐藏闪烁的光标
	ShowCursor   = Escape + "[?25h"   // 重新显示光标
	ClearScreen  = Escape + "[2J"     // 清空整个屏幕
	CursorHome   = Escape + "[H"      // 把光标移到左上角 (1,1)
	ColorReset   = Escape + "[0m"     // 重置所有颜色/样式
)

// Visualizer 是界面的"导演"——它不亲自画任何东西，而是创建组件并把它们摆到终端上
// 真正的绘制逻辑都在 component.go 里的各个组件中
type Visualizer struct {
	width  int    // 终端当前宽度（列数）
	height int    // 终端当前高度（行数）
	pink   string // 预设的粉红色 —— 用于边框标题
	green  string // 预设的绿色 —— 暂未使用，留着扩展
	cyan   string // 预设的青色 —— 用于进度条

	// 三个界面组件，每帧 Render 时重新创建（布局可随终端尺寸动态变化）
	box       *BoxComponent       // 外围边框
	statsList *StatsListComponent // 按键排行榜
	footer    *TextComponent      // 底部提示文字
}

// NewVisualizer 创建一个可视化器，顺便定义好三种常用颜色
func NewVisualizer() *Visualizer {
	return &Visualizer{
		pink:  fgRGB(230, 50, 100), // 粉红
		green: fgRGB(50, 200, 100), // 绿色
		cyan:  fgRGB(0, 255, 255),  // 青色
	}
}

// Init 初始化终端：切到备用屏幕、隐藏光标、把终端设为 raw 模式
// raw 模式的意思是按键不再等待回车，一按就能读到，适合实时交互
// 返回值 oldState 保存了进入 raw 模式前的终端状态，退出时需要用它恢复
func (v *Visualizer) Init() (*term.State, error) {
	fmt.Print(AltScreenOn + HideCursor)
	return term.MakeRaw(int(os.Stdin.Fd()))
}

// Cleanup 程序退出时调用，把终端恢复成原样
func (v *Visualizer) Cleanup(oldState *term.State) {
	term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Print(AltScreenOff + ShowCursor)
}

// Render 是核心函数 —— 每帧调用一次，把 stats 数据画成完整的界面
// 它不亲自画任何东西，而是：创建组件 → 设置数据 → 让组件自己渲染
func (v *Visualizer) Render(stats map[uint16]int) {
	// 每次渲染前重新获取终端尺寸，窗口缩放时能自适应
	v.width, v.height, _ = term.GetSize(int(os.Stdout.Fd()))

	var b strings.Builder

	// 先清屏、光标归位，确保接下来画的内容覆盖整屏
	b.WriteString(ClearScreen + CursorHome)

	// 1. 画外围边框 —— 类似 btop 的外框，标题显示在左上角
	v.box = NewBoxComponent(2, 2, v.width-2, v.height-2, " Key Counter Pro (btop-style) ", v.pink)
	v.box.Render(&b)

	// 2. 画按键排行榜 —— 在边框内部留出边距
	v.statsList = NewStatsListComponent(4, 4, v.width-6, v.height-6, v.cyan)
	v.statsList.Stats = stats // 把数据塞进组件
	v.statsList.Render(&b)

	// 3. 底部提示文字
	v.footer = NewTextComponent(4, v.height-1, "Press 'q' or Ctrl+C to quit | Monitoring Global Input...", fgRGB(150, 150, 150))
	v.footer.Render(&b)

	// 一次性输出全部内容
	fmt.Print(b.String())
}

// fgRGB 生成设置前景色为指定 RGB 值的 ANSI 控制码
// 这是 24-bit 真彩色，现代终端都支持
func fgRGB(r, g, b int) string {
	return fmt.Sprintf("%s[38;2;%d;%d;%dm", Escape, r, g, b)
}
