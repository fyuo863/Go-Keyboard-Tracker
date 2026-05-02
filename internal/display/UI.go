// Package display 负责构建和控制终端 UI 界面（基于 tview 组件库）。
package display

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// KeyEntry 带时间戳的按键记录，用于 Input Queue 的时基渐隐显示。
type KeyEntry struct {
	Name string
	At   time.Time
}

// tabHitArea 记录标签页标题在屏幕上的起止位置，
// 用于判断鼠标点击落在了哪个标签上。
type tabHitArea struct {
	Start int // 该标签在 TabBar 文本中的起始列
	End   int // 该标签在 TabBar 文本中的结束列
	Index int // 对应的标签页序号
}

// DashboardUI 是整个键盘监测仪表盘的主界面结构体，
// 把所有子组件（标题、标签栏、页面区、页脚）组合在一起。
// 所有方法均在 tview 事件循环 goroutine 中运行，无需加锁。
type DashboardUI struct {
	Root           tview.Primitive   // 顶层布局，传递给 tview.Application
	Pages          *tview.Pages      // 页面切换容器，容纳 tab1 / tab2
	TabBar         *tview.TextView   // 顶部标签栏（渲染彩色标签名）
	StatsTable     *tview.Table      // 按键统计表（tab2 内容）
	InputQueueView *tview.TextView   // 最近按键队列视图
	BrailleGraph   *BrailleGraphView // 盲文曲线图（tab1 内容）
	Footer         *tview.TextView   // 底部操作提示栏

	activeTab int          // 当前激活的标签页编号
	tabNames  []string     // 所有标签页的页面名称列表
	hitAreas  []tabHitArea // 当前标签文字的可点击区域缓存
}

// BuildDashboard 构建整个仪表盘 UI 并返回。
// 布局从上到下依次为：标题行 → 标签栏 → 页面内容区 → 页脚提示行。
func BuildDashboard() *DashboardUI {
	// —— 标题行 ——
	title := tview.NewTextView()
	title.SetTextAlign(tview.AlignCenter)
	title.SetDynamicColors(true)
	title.SetText("[::b]Key Counter Pro (btop-style) [::-]")
	title.SetTextColor(rgb(230, 50, 100))
	//title.SetBorder(true)
	//title.SetBorderColor(rgb(230, 50, 100))

	// —— 标签栏（tab1 / tab2 的切换区域）——
	tabBar := tview.NewTextView()
	tabBar.SetDynamicColors(true)
	tabBar.SetRegions(false) // 开启区域支持，用于标签鼠标点击检测
	tabBar.SetWrap(false)    // 不换行，标签在一行内显示
	tabBar.SetBorder(true)
	tabBar.SetBorderColor(rgb(94, 92, 100))
	//tabBar.SetTitle(" Tabs ")

	// —— Tab1：盲文曲线 + 按键队列 ——
	graph := NewBrailleGraphView()
	queueView := tview.NewTextView()
	queueView.SetDynamicColors(true)
	queueView.SetWrap(false) // 单行显示，超出部分自然截断
	queueView.SetBorder(true)
	queueView.SetTitle(" Input Queue ")
	queueView.SetBorderColor(rgb(114, 159, 207))
	queueView.SetTitleColor(rgb(114, 159, 207))

	tab1 := tview.NewFlex().SetDirection(tview.FlexRow)
	tab1.AddItem(graph, 0, 5, true)      // 曲线图占 5 份高度
	tab1.AddItem(queueView, 3, 0, false) // 队列固定 1 行

	// —— Tab2：按键次数统计表 ——
	statsTable := tview.NewTable()
	statsTable.SetBorder(true)
	statsTable.SetTitle(" Key Counter ")
	statsTable.SetBorderColor(rgb(230, 50, 100))
	statsTable.SetTitleColor(rgb(230, 50, 100))
	statsTable.SetSelectable(false, false) // 禁止单元格选中

	// —— 页面容器：管理 tab1 / tab2 的显示与隐藏 ——
	pages := tview.NewPages()
	pages.AddPage("tab1", tab1, true, true)        // 默认显示 tab1
	pages.AddPage("tab2", statsTable, true, false) // 默认隐藏 tab2

	// —— 页脚操作提示 ——
	footer := tview.NewTextView()
	footer.SetDynamicColors(true)
	footer.SetText("←/→ or 1/2 switch tabs | Esc / Ctrl+C quit")
	footer.SetTextColor(rgb(150, 150, 150))

	// —— 组装根布局 ——
	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.AddItem(title, 1, 0, false)  // 标题固定 1 行
	root.AddItem(tabBar, 3, 0, false) // 标签栏固定 3 行
	root.AddItem(pages, 0, 1, true)   // 页面区占满剩余空间
	root.AddItem(footer, 1, 0, false) // 页脚固定 1 行

	// 外层隐形容器：上下左右各 2 格页边距
	outerWrapper := tview.NewFlex().SetDirection(tview.FlexRow)
	outerWrapper.AddItem(nil, 0, 0, false) // 上边距
	contentRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	contentRow.AddItem(nil, 2, 0, false) // 左边距
	contentRow.AddItem(root, 0, 1, true) // 内容区
	contentRow.AddItem(nil, 2, 0, false) // 右边距
	outerWrapper.AddItem(contentRow, 0, 1, true)
	outerWrapper.AddItem(nil, 2, 0, false) // 下边距

	ui := &DashboardUI{
		Root:           outerWrapper,
		Pages:          pages,
		TabBar:         tabBar,
		StatsTable:     statsTable,
		InputQueueView: queueView,
		BrailleGraph:   graph,
		Footer:         footer,
		activeTab:      0,
		tabNames:       []string{"tab1", "tab2"},
	}

	// 初始化标签栏渲染 & 鼠标点击处理 & 各组件默认状态
	ui.renderTabs()
	ui.installTabMouseHandler()
	RefreshInputQueue(queueView, nil)
	RefreshBrailleGraph(graph, nil)

	return ui
}

// SetActiveTab 切换到指定序号的标签页，同时高亮对应标签文字。
func (ui *DashboardUI) SetActiveTab(index int) {
	if index < 0 || index >= len(ui.tabNames) {
		return // 序号非法，直接忽略
	}
	if index == ui.activeTab {
		return // 已在目标页，无需切换
	}
	ui.activeTab = index
	// SwitchToPage 原子性地完成：显示目标页 + 隐藏其余页 + 重新聚焦
	ui.Pages.SwitchToPage(ui.tabNames[index])
	ui.renderTabsLocked() // 重绘标签栏高亮
}

// NextTab 切换到下一个标签页（循环）。
func (ui *DashboardUI) NextTab() {
	next := (ui.activeTab + 1) % len(ui.tabNames)
	ui.SetActiveTab(next)
}

// PrevTab 切换到上一个标签页（循环）。
func (ui *DashboardUI) PrevTab() {
	prev := (ui.activeTab - 1 + len(ui.tabNames)) % len(ui.tabNames)
	ui.SetActiveTab(prev)
}

// renderTabs 绘制标签栏文字与点击区域缓存。
func (ui *DashboardUI) renderTabs() {
	ui.renderTabsLocked()
}

// renderTabsLocked 构建标签栏文本并缓存每个标签的点击区域。
// 调用方需持有 ui.mu 写锁。
func (ui *DashboardUI) renderTabsLocked() {
	var builder strings.Builder
	hitAreas := make([]tabHitArea, 0, len(ui.tabNames))
	cursor := 0
	for i, name := range ui.tabNames {
		// 标签之间加两个空格分隔
		if i > 0 {
			separator := "  "
			builder.WriteString(separator)
			cursor += len(separator)
		}

		// 当前选中标签用绿底黑字高亮，其余用蓝色文字
		label := fmt.Sprintf(" %s ", name)
		start := cursor
		if i == ui.activeTab {
			builder.WriteString(fmt.Sprintf("[black:#8ae234]%s[-:-:-]", label))
		} else {
			builder.WriteString(fmt.Sprintf("[#729fcf]%s[-:-:-]", label))
		}
		cursor += len(label)
		// 记录该标签在文本中的列范围，供鼠标点击命中测试使用
		hitAreas = append(hitAreas, tabHitArea{Start: start, End: cursor, Index: i})
	}
	ui.hitAreas = hitAreas
	ui.TabBar.SetText(builder.String())
}

// installTabMouseHandler 为标签栏绑定鼠标左键点击事件，
// 点击标签文字即可切换到对应页面。
func (ui *DashboardUI) installTabMouseHandler() {
	ui.TabBar.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		// 只处理鼠标左键点击
		if action != tview.MouseLeftClick {
			return action, event
		}
		x, y := event.Position()
		rectX, rectY, _, _ := ui.TabBar.GetInnerRect()
		// 点击位置必须在标签栏所在行
		if y != rectY {
			return action, event
		}
		column := x - rectX

		// 遍历点击区域判断落在哪个标签上
		for _, area := range ui.hitAreas {
			if column >= area.Start && column < area.End {
				ui.SetActiveTab(area.Index)
				return action, nil // 消费该事件
			}
		}
		return action, event
	})
}

// RefreshInputQueue 刷新按键队列视图。
// 从左到右排列，最新按键在最左侧，每个按键根据其按下后的经过时间逐渐变暗。
// 亮青色(0,255,255) 经过 3 秒线性衰减到暗灰色(0,51,51) 后亮度不再降低。
// recent 按时间升序排列（最早在前 index=0，最新在末尾）。
func RefreshInputQueue(view *tview.TextView, recent []KeyEntry) {
	if len(recent) == 0 {
		view.SetText("[gray]Waiting for keyboard input...")
		return
	}
	var builder strings.Builder
	now := time.Now()
	maxAge := 3.0
	n := len(recent)
	for i := n - 1; i >= 0; i-- {
		elapsed := now.Sub(recent[i].At).Seconds()
		if elapsed < 0 {
			elapsed = 0
		}
		t := elapsed / maxAge
		if t > 1.0 {
			t = 1.0
		}
		v := int(255 - t*204)
		fmt.Fprintf(&builder, "[#00%02x%02x]%s", v, v, recent[i].Name)
		if i > 0 {
			builder.WriteString(" ")
		}
	}
	builder.WriteString("[-]")
	view.SetText(builder.String())
}

// RefreshBrailleGraph 刷新盲文曲线图的采样数据并触发重绘。
func RefreshBrailleGraph(graph *BrailleGraphView, samples []int) {
	graph.SetSamples(samples)
}

// RefreshTable 根据按键统计数据刷新表格。
// stats 的 key 是扫描码，value 是按键次数。
func RefreshTable(table *tview.Table, stats map[uint16]int) {
	// 用于排序的键值对结构
	type kv struct {
		Key   uint16
		Count int
	}
	var sorted []kv
	for k, v := range stats {
		sorted = append(sorted, kv{k, v})
	}
	// 按按键次数降序排列，次数相同时按键码升序
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count == sorted[j].Count {
			return sorted[i].Key < sorted[j].Key
		}
		return sorted[i].Count > sorted[j].Count
	})

	table.Clear()

	// 表头：按键 | 次数 | 频率
	headerColor := rgb(255, 255, 0)
	table.SetCell(0, 0, tview.NewTableCell("Key").SetTextColor(headerColor).SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("Count").SetTextColor(headerColor).SetSelectable(false))
	table.SetCell(0, 2, tview.NewTableCell("Frequency").SetTextColor(headerColor).SetSelectable(false))

	cyan := rgb(138, 226, 52)
	barLen := 20 // 频率进度条的长度（字符数）
	maxCount := 0
	for _, item := range sorted {
		if item.Count > maxCount {
			maxCount = item.Count
		}
	}
	if maxCount == 0 {
		maxCount = 1 // 避免除零
	}

	// 逐行填充数据
	for i, item := range sorted {
		row := i + 1
		table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("Key 0x%02X", item.Key)))
		table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", item.Count)))

		// 绘制频率进度条，用 █ 填充 + 百分比数字
		barPercent := item.Count * 100 / maxCount
		fillLen := barPercent * barLen / 100
		bar := fmt.Sprintf("[%s%s] %d%%",
			strings.Repeat("█", fillLen),
			strings.Repeat(" ", barLen-fillLen),
			barPercent,
		)
		table.SetCell(row, 2, tview.NewTableCell(bar).SetTextColor(cyan))
	}
}

// BrailleGraphView 是一个自定义的 tview 组件，
// 用 Unicode 盲文字符 (U+2800~U+28FF) 在终端中绘制按键频率曲线图。
// 每个盲文字符包含 2×4 个点，相比普通柱状图精度更高。
// SetSamples 和 Draw 均在事件循环同一 goroutine 中调用，无需加锁。
type BrailleGraphView struct {
	*tview.Box
	samples []int // 各时间桶的按键次数数据
}

// NewBrailleGraphView 创建盲文曲线图组件并设置边框样式。
func NewBrailleGraphView() *BrailleGraphView {
	box := tview.NewBox()
	box.SetBorder(true)
	box.SetTitle(" Braille Frequency Curve (30s) ")
	box.SetBorderColor(rgb(138, 226, 52))
	box.SetTitleColor(rgb(138, 226, 52))
	return &BrailleGraphView{Box: box}
}

// SetSamples 更新曲线图采样数据。
func (g *BrailleGraphView) SetSamples(samples []int) {
	g.samples = append([]int(nil), samples...)
}

// Draw 是自定义绘制方法，tview 会在刷新时自动调用。
// 它将 samples 缩放后映射到盲文字符网格上进行渲染。
func (g *BrailleGraphView) Draw(screen tcell.Screen) {
	g.Box.DrawForSubclass(screen, g) // 先绘制边框
	x, y, width, height := g.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	// 获取采样数据的副本
	samples := append([]int(nil), g.samples...)

	style := tcell.StyleDefault.Foreground(rgb(138, 226, 52))
	textStyle := tcell.StyleDefault.Foreground(rgb(150, 150, 150))
	if len(samples) == 0 {
		tview.Print(screen, "Waiting for 30s bucket...", x, y+height/2, width, tview.AlignCenter, rgb(150, 150, 150))
		return
	}

	// 每个字符列可容纳 2 个采样点（盲文字符左右两列），
	// 整个 box 宽度可容纳 width*2 个采样点
	maxCols := width * 2
	if maxCols < len(samples) {
		samples = samples[len(samples)-maxCols:]
	}
	maxValue := 0
	for _, v := range samples {
		if v > maxValue {
			maxValue = v
		}
	}
	if maxValue == 0 {
		tview.Print(screen, "No key presses in recent buckets", x, y+height/2, width, tview.AlignCenter, rgb(150, 150, 150))
		return
	}

	// 构建高精度网格：高度 = 字符行数 × 4，宽度 = 字符列数 × 2
	cells := make([][]int, height*4)
	for i := range cells {
		cells[i] = make([]int, width*2)
	}

	// 曲线从最右侧开始绘制：计算左侧偏移量，使数据右对齐
	offset := maxCols - len(samples)
	for i, sample := range samples {
		col := offset + i
		scaled := int(math.Round(float64(sample) / float64(maxValue) * float64(height*4)))
		if scaled < 1 && sample > 0 {
			scaled = 1
		}
		for level := 0; level < scaled && level < height*4; level++ {
			row := height*4 - 1 - level
			cells[row][col] = 1
		}
	}

	// 将网格转换为盲文字符并绘制到屏幕
	for cellY := 0; cellY < height; cellY++ {
		for cellX := 0; cellX < width; cellX++ {
			pattern := braillePattern(cells, cellY, cellX)
			ch := rune(0x2800 + pattern)
			if pattern == 0 {
				ch = ' '
			}
			screen.SetContent(x+cellX, y+cellY, ch, nil, style)
		}
	}

	// 在 box 内部左上角标注最大值
	label := fmt.Sprintf("max %d keys", maxValue)
	tview.Print(screen, label, x, y, len(label), tview.AlignLeft, rgb(114, 159, 207))
	_ = textStyle
}

// braillePattern 计算字符位置 (cellY, cellX) 对应的盲文点阵编码。
// 盲文字符 Unicode 编码规则：U+2800 开始，每个字符有 8 个点，
// 按 1-8 号点位对应 0-7 bit。
//
//	点位映射（标准盲文布局）：
//	点1(bit0) 点4(bit3)
//	点2(bit1) 点5(bit4)
//	点3(bit2) 点6(bit5)
//	点7(bit6) 点8(bit7)
func braillePattern(cells [][]int, cellY, cellX int) int {
	baseRow := cellY * 4
	baseCol := cellX * 2
	bits := [8]struct {
		Row int
		Col int
		Bit int
	}{
		{0, 0, 0}, // 点1 - 左上
		{1, 0, 1}, // 点2 - 左中
		{2, 0, 2}, // 点3 - 左下
		{0, 1, 3}, // 点4 - 右上
		{1, 1, 4}, // 点5 - 右中
		{2, 1, 5}, // 点6 - 右下
		{3, 0, 6}, // 点7 - 左下额外
		{3, 1, 7}, // 点8 - 右下额外
	}

	pattern := 0
	for _, bit := range bits {
		row := baseRow + bit.Row
		col := baseCol + bit.Col
		if row >= 0 && row < len(cells) && col >= 0 && col < len(cells[row]) && cells[row][col] != 0 {
			pattern |= 1 << bit.Bit // 该点位有数据，置对应 bit
		}
	}
	return pattern
}

// rgb 将 RGB 颜色值（0-255）转换为 tcell.Color。
func rgb(r, g, b int) tcell.Color {
	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}
