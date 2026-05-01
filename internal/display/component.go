package display

import (
	"fmt"
	"sort"
	"strings"
)

// ═══════════════════════════════════════════════════════════
// 基础工具函数
// ═══════════════════════════════════════════════════════════

// moveCursor 生成把光标移到 (x, y) 位置的控制码
// 注意：ANSI 坐标是 (行, 列)，也就是 (y, x)
func moveCursor(x, y int) string {
	return fmt.Sprintf("%s[%d;%dH", Escape, y, x)
}

// ═══════════════════════════════════════════════════════════
// 基础组件 —— 所有组件的公共属性
// ═══════════════════════════════════════════════════════════

// BaseComponent 定义了每个组件都有的位置和尺寸
// 其他组件通过嵌入它来获得 X/Y/Width/Height 属性
type BaseComponent struct {
	X, Y          int // 左上角坐标（列号, 行号）
	Width, Height int // 组件占用的宽高（列数, 行数）
}

// ═══════════════════════════════════════════════════════════
// 盒子组件 —— 带 Unicode 框线的矩形边框
// ═══════════════════════════════════════════════════════════

type BoxComponent struct {
	BaseComponent        // 嵌入基础位置属性
	Title         string // 标题文字，显示在顶部边框上
	Color         string // 边框的 ANSI 颜色码
}

// NewBoxComponent 创建一个盒子组件
func NewBoxComponent(x, y, w, h int, title, color string) *BoxComponent {
	return &BoxComponent{
		BaseComponent: BaseComponent{X: x, Y: y, Width: w, Height: h},
		Title:         title,
		Color:         color,
	}
}

// Render 把盒子"画"进 strings.Builder
func (c *BoxComponent) Render(b *strings.Builder) {
	b.WriteString(c.Color)

	// 顶部边框：╭───...───╮
	b.WriteString(moveCursor(c.X, c.Y))
	b.WriteString("╭" + strings.Repeat("─", c.Width-2) + "╮")

	// 如果有标题，盖在顶部边框的左侧
	if c.Title != "" {
		b.WriteString(moveCursor(c.X+2, c.Y))
		b.WriteString(" " + c.Title + " ")
	}

	// 左右两侧的竖线
	for i := 1; i < c.Height-1; i++ {
		b.WriteString(moveCursor(c.X, c.Y+i) + "│")          // 左边框
		b.WriteString(moveCursor(c.X+c.Width-1, c.Y+i) + "│") // 右边框
	}

	// 底部边框：╰───...───╯
	b.WriteString(moveCursor(c.X, c.Y+c.Height-1))
	b.WriteString("╰" + strings.Repeat("─", c.Width-2) + "╯")

	b.WriteString(ColorReset)
}

// ═══════════════════════════════════════════════════════════
// 进度条组件 —— 类似 [████████        ] 75
// ═══════════════════════════════════════════════════════════

type ProgressBarComponent struct {
	BaseComponent             // 只用 X, Y，Width/Height 闲置
	Percent       int         // 百分比 (0~100)
	Color         string      // 进度条的 ANSI 颜色码
	BarLen        int         // 总共几格，默认 20
}

// NewProgressBarComponent 创建一个进度条组件
func NewProgressBarComponent(x, y, percent int, color string) *ProgressBarComponent {
	return &ProgressBarComponent{
		BaseComponent: BaseComponent{X: x, Y: y},
		Percent:       percent,
		Color:         color,
		BarLen:        20,
	}
}

// Render 把进度条"画"进 strings.Builder
func (c *ProgressBarComponent) Render(b *strings.Builder) {
	b.WriteString(moveCursor(c.X, c.Y))
	b.WriteString(c.Color + "[")

	// 按百分比算出填充格数
	fillLen := c.Percent * c.BarLen / 100
	if fillLen > c.BarLen {
		fillLen = c.BarLen
	}

	b.WriteString(strings.Repeat("█", fillLen))              // 已填充部分
	b.WriteString(strings.Repeat(" ", c.BarLen-fillLen))     // 剩余空白部分
	fmt.Fprintf(b, "] %d", c.Percent)                        // 结尾显示数字

	b.WriteString(ColorReset)
}

// ═══════════════════════════════════════════════════════════
// 排行榜组件 —— 显示按键次数排行，每行带进度条
// ═══════════════════════════════════════════════════════════

type StatsListComponent struct {
	BaseComponent            // 嵌入基础位置属性
	Stats         map[uint16]int // 按键统计数据，由外部在 Render 前赋值
	BarColor      string         // 传递给每行进度条的颜色
}

// NewStatsListComponent 创建一个排行榜组件
func NewStatsListComponent(x, y, w, h int, barColor string) *StatsListComponent {
	return &StatsListComponent{
		BaseComponent: BaseComponent{X: x, Y: y, Width: w, Height: h},
		BarColor:      barColor,
	}
}

// Render 把排行榜"画"进 strings.Builder
func (c *StatsListComponent) Render(b *strings.Builder) {
	// 临时结构体，用于排序
	type kv struct {
		Key   uint16
		Count int
	}

	// 把 map 转成切片，按次数从高到低排序
	var sorted []kv
	for k, val := range c.Stats {
		sorted = append(sorted, kv{k, val})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	// 能显示的最大行数
	maxRows := c.Height - 2
	for i, item := range sorted {
		if i >= maxRows {
			break // 超出可视区域，不画了
		}
		rowY := c.Y + i + 1

		// 键名（如 "Key 0x41" 对应键盘上的 A 键）
		b.WriteString(moveCursor(c.X+2, rowY))
		keyName := fmt.Sprintf("Key 0x%02X", item.Key)
		b.WriteString(fmt.Sprintf("%-12s ", keyName))

		// 计算百分比（当前直接用次数，超过 100 就顶满）
		barPercent := item.Count
		if barPercent > 100 {
			barPercent = 100
		}

		// 每行临时创建一个进度条来画 —— 栈上分配，不产生 GC 压力
		bar := ProgressBarComponent{
			BaseComponent: BaseComponent{X: c.X + 15, Y: rowY},
			Percent:       barPercent,
			Color:         c.BarColor,
			BarLen:        20,
		}
		bar.Render(b)
	}
}

// ═══════════════════════════════════════════════════════════
// 文本组件 —— 最简单的组件，在指定位置显示一行文字
// ═══════════════════════════════════════════════════════════

type TextComponent struct {
	BaseComponent         // 只用 X, Y
	Text          string  // 要显示的文字
	Color         string  // 文字的 ANSI 颜色码
}

// NewTextComponent 创建一个文本组件
func NewTextComponent(x, y int, text, color string) *TextComponent {
	return &TextComponent{
		BaseComponent: BaseComponent{X: x, Y: y},
		Text:          text,
		Color:         color,
	}
}

// Render 把文字"画"进 strings.Builder
func (c *TextComponent) Render(b *strings.Builder) {
	b.WriteString(moveCursor(c.X, c.Y))
	b.WriteString(c.Color + c.Text + ColorReset)
}
