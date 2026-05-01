package display

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/term"
)

// ═══════════════════════════════════════════════════════════
// ANSI 终端控制码 —— 相当于给终端发送的"指令"，用来清屏、移动光标、变色等
// ═══════════════════════════════════════════════════════════

const (
	Escape       = "\033"             // ESC 字符，所有控制码的开头
	AltScreenOn  = Escape + "[?1049h" // 切换到"备用屏幕"（退出后原终端内容还在）
	AltScreenOff = Escape + "[?1049l" // 切回原屏幕
	HideCursor   = Escape + "[?25l"   // 隐藏闪烁的光标
	ShowCursor   = Escape + "[?25h"   // 重新显示光标
	ClearScreen  = Escape + "[2J"     // 清空整个屏幕
	CursorHome   = Escape + "[H"      // 把光标移到左上角 (1,1)
	ColorReset   = Escape + "[0m"     // 重置所有颜色/样式
)

// Visualizer 负责把数据"画"到终端上
// 它不关心键盘数据怎么来的，只管拿到数据后渲染成 btop 风格的界面
type Visualizer struct {
	width  int    // 终端当前宽度（列数）
	height int    // 终端当前高度（行数）
	pink   string // 预设的粉红色 ANSI 码
	green  string // 预设的绿色 ANSI 码
	cyan   string // 预设的青色 ANSI 码
}

// NewVisualizer 创建一个可视化器，顺便定义好三种常用颜色
func NewVisualizer() *Visualizer {
	return &Visualizer{
		pink:  fgRGB(230, 50, 100), // 粉红 —— 用于边框标题
		green: fgRGB(50, 200, 100), // 绿色 —— 暂未使用，留着扩展
		cyan:  fgRGB(0, 255, 255),  // 青色 —— 用于进度条
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
// 如果不恢复，终端可能会看起来"坏掉了"（没有光标、行缓冲失效等）
func (v *Visualizer) Cleanup(oldState *term.State) {
	term.Restore(int(os.Stdin.Fd()), oldState)
	fmt.Print(AltScreenOff + ShowCursor)
}

// Render 是核心函数 —— 每帧调用一次，把 stats 数据画成完整的界面
// stats 的 key 是按键的虚拟键码，value 是该按键被按下的次数
func (v *Visualizer) Render(stats map[uint16]int) {
	// 每次渲染前重新获取终端尺寸，窗口缩放时能自适应
	v.width, v.height, _ = term.GetSize(int(os.Stdout.Fd()))

	// 用 strings.Builder 拼接所有输出，最后一次性打印（比多次 fmt.Print 效率高）
	var b strings.Builder

	// 先清屏、光标归位，确保接下来画的内容覆盖整屏
	b.WriteString(ClearScreen + CursorHome)

	// 1. 画外围边框 —— 类似 btop 的外框，标题显示在左上角
	v.drawBox(&b, 2, 2, v.width-2, v.height-2, " Key Counter Pro (btop-style) ", v.pink)

	// 2. 在边框内部画按键排行榜
	v.drawStatsList(&b, 4, 4, v.width-6, v.height-6, stats)

	// 3. 最底部画一行灰色提示文字，告诉用户怎么退出
	b.WriteString(v.moveCursor(4, v.height-1))
	b.WriteString(fgRGB(150, 150, 150) + " Press Ctrl+C to quit " + ColorReset)

	// 一次性输出全部内容
	fmt.Print(b.String())
}

// drawStatsList 在指定矩形区域内绘制按键次数排行榜
// x, y 是区域左上角坐标，w, h 是区域的宽和高
func (v *Visualizer) drawStatsList(b *strings.Builder, x, y, w, h int, stats map[uint16]int) {
	// --- 把 map 转成切片，按次数从高到低排序 ---
	type kv struct {
		Key   uint16
		Count int
	}
	var sorted []kv
	for k, val := range stats {
		sorted = append(sorted, kv{k, val})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count == sorted[j].Count {
			// 次数相同，按 Key 升序排列（小的在前）
			return sorted[i].Key < sorted[j].Key
		}
		return sorted[i].Count > sorted[j].Count // 降序：次数多的在前面
	})

	// 能显示的最大行数 = 区域高度 - 2（上下各留一行边距）
	maxRows := h - 2
	for i, item := range sorted {
		if i >= maxRows {
			break // 超出可视区域，不画了
		}
		rowY := y + i + 1 // 当前行在终端里的 Y 坐标

		// 把光标移到这一行的起始位置
		b.WriteString(v.moveCursor(x+2, rowY))

		// 显示键名（如 "Key 0x41" 对应键盘上的 A 键）
		keyName := fmt.Sprintf("Key 0x%02X", item.Key)
		b.WriteString(fmt.Sprintf("%-12s ", keyName)) // 左对齐占 12 格，保持对齐

		// 把次数直接当作百分比来画进度条（待优化：应按"当前最大值"来算比例）
		barPercent := (item.Count * 100) / 100
		if barPercent > 100 {
			barPercent = 100 // 超过 100 就顶满格子
		}
		v.drawProgressBar(b, x+15, rowY, barPercent, v.cyan)
	}
}

// ═══════════════════════════════════════════════════════════
// 下面是基础的"画图积木"——不关心业务，只负责生成 ANSI 字符串
// ═══════════════════════════════════════════════════════════

// moveCursor 生成把光标移到 (x, y) 位置的控制码
// 注意：ANSI 坐标是 (行, 列)，也就是 (y, x)，所以 f 字符串里 y 在前
func (v *Visualizer) moveCursor(x, y int) string {
	return fmt.Sprintf("%s[%d;%dH", Escape, y, x)
}

// drawBox 画一个用 Unicode 框线字符组成的矩形边框，可选标题
// x, y 是边框左上角，width/height 是边框的宽高
func (v *Visualizer) drawBox(b *strings.Builder, x, y, width, height int, title string, color string) {
	b.WriteString(color)

	// 顶部边框：╭───...───╮
	b.WriteString(v.moveCursor(x, y))
	b.WriteString("╭" + strings.Repeat("─", width-2) + "╮")

	// 如果有标题，盖在顶部边框的左侧
	if title != "" {
		b.WriteString(v.moveCursor(x+2, y))
		b.WriteString(" " + title + " ")
	}

	// 左右两侧的竖线：│               │
	for i := 1; i < height-1; i++ {
		b.WriteString(v.moveCursor(x, y+i) + "│")         // 左边框
		b.WriteString(v.moveCursor(x+width-1, y+i) + "│") // 右边框
	}

	// 底部边框：╰───...───╯
	b.WriteString(v.moveCursor(x, y+height-1))
	b.WriteString("╰" + strings.Repeat("─", width-2) + "╯")

	b.WriteString(ColorReset) // 恢复默认颜色，不影响后面的输出
}

// drawProgressBar 画一个 20 格宽的进度条，类似 [████████        ] 75
// x, y 是进度条起始位置，percent 是 0~100 的百分比
func (v *Visualizer) drawProgressBar(b *strings.Builder, x, y int, percent int, color string) {
	b.WriteString(v.moveCursor(x, y))
	b.WriteString(color + "[")

	barLen := 20 // 总共 20 格
	// 按百分比算出应该填充几格
	fillLen := int(float64(percent) / 100.0 * float64(barLen))
	b.WriteString(strings.Repeat("█", fillLen))        // 已填充部分
	b.WriteString(strings.Repeat("░", barLen-fillLen)) // 剩余空白部分
	fmt.Fprintf(b, "] %d", percent)                    // 结尾显示数字

	b.WriteString(ColorReset)
}

// fgRGB 生成设置前景色为指定 RGB 值的 ANSI 控制码
// 这是 24-bit 真彩色，现代终端都支持
func fgRGB(r, g, b int) string {
	return fmt.Sprintf("%s[38;2;%d;%d;%dm", Escape, r, g, b)
}
