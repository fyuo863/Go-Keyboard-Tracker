package display

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type tabHitArea struct {
	Start int
	End   int
	Index int
}

type DashboardUI struct {
	Root           tview.Primitive
	Pages          *tview.Pages
	TabBar         *tview.TextView
	StatsTable     *tview.Table
	InputQueueView *tview.TextView
	BrailleGraph   *BrailleGraphView
	Footer         *tview.TextView

	activeTab int
	tabNames  []string
	hitAreas  []tabHitArea
	mu        sync.RWMutex
}

func BuildDashboard() *DashboardUI {
	title := tview.NewTextView()
	title.SetTextAlign(tview.AlignCenter)
	title.SetDynamicColors(true)
	title.SetText("[::b]Tit[::-]")
	title.SetTextColor(rgb(230, 50, 100))
	title.SetBorder(true)
	title.SetBorderColor(rgb(230, 50, 100))

	tabBar := tview.NewTextView()
	tabBar.SetDynamicColors(true)
	tabBar.SetRegions(true)
	tabBar.SetWrap(false)
	tabBar.SetBorder(true)
	tabBar.SetBorderColor(rgb(94, 92, 100))
	tabBar.SetTitle(" Tabs ")

	graph := NewBrailleGraphView()
	queueView := tview.NewTextView()
	queueView.SetDynamicColors(true)
	queueView.SetWrap(false)
	queueView.SetBorder(true)
	queueView.SetTitle(" Input Queue ")
	queueView.SetBorderColor(rgb(114, 159, 207))
	queueView.SetTitleColor(rgb(114, 159, 207))

	tab1 := tview.NewFlex().SetDirection(tview.FlexRow)
	tab1.AddItem(graph, 0, 4, true)
	tab1.AddItem(queueView, 8, 0, false)

	statsTable := tview.NewTable()
	statsTable.SetBorder(true)
	statsTable.SetTitle(" Key Counter Pro (btop-style) ")
	statsTable.SetBorderColor(rgb(230, 50, 100))
	statsTable.SetTitleColor(rgb(230, 50, 100))
	statsTable.SetSelectable(false, false)

	pages := tview.NewPages()
	pages.AddPage("tab1", tab1, true, true)
	pages.AddPage("tab2", statsTable, true, false)

	footer := tview.NewTextView()
	footer.SetDynamicColors(true)
	footer.SetText("←/→ or 1/2 switch tabs | Click tab labels | q / Esc / Ctrl+C quit")
	footer.SetTextColor(rgb(150, 150, 150))

	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.AddItem(title, 3, 0, false)
	root.AddItem(tabBar, 3, 0, false)
	root.AddItem(pages, 0, 1, true)
	root.AddItem(footer, 1, 0, false)

	ui := &DashboardUI{
		Root:           root,
		Pages:          pages,
		TabBar:         tabBar,
		StatsTable:     statsTable,
		InputQueueView: queueView,
		BrailleGraph:   graph,
		Footer:         footer,
		activeTab:      0,
		tabNames:       []string{"tab1", "tab2"},
	}

	ui.renderTabs()
	ui.installTabMouseHandler()
	RefreshInputQueue(queueView, nil)
	RefreshBrailleGraph(graph, nil)

	return ui
}

func (ui *DashboardUI) SetActiveTab(index int) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	if index < 0 || index >= len(ui.tabNames) {
		return
	}
	ui.activeTab = index
	for i, name := range ui.tabNames {
		ui.Pages.SwitchToPage(name)
		ui.Pages.HidePage(name)
		if i == index {
			ui.Pages.ShowPage(name)
			ui.Pages.SwitchToPage(name)
		}
	}
	ui.renderTabsLocked()
}

func (ui *DashboardUI) NextTab() {
	ui.mu.RLock()
	next := (ui.activeTab + 1) % len(ui.tabNames)
	ui.mu.RUnlock()
	ui.SetActiveTab(next)
}

func (ui *DashboardUI) PrevTab() {
	ui.mu.RLock()
	prev := (ui.activeTab - 1 + len(ui.tabNames)) % len(ui.tabNames)
	ui.mu.RUnlock()
	ui.SetActiveTab(prev)
}

func (ui *DashboardUI) renderTabs() {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	ui.renderTabsLocked()
}

func (ui *DashboardUI) renderTabsLocked() {
	var builder strings.Builder
	hitAreas := make([]tabHitArea, 0, len(ui.tabNames))
	cursor := 0
	for i, name := range ui.tabNames {
		if i > 0 {
			separator := "  "
			builder.WriteString(separator)
			cursor += len(separator)
		}

		label := fmt.Sprintf(" %s ", name)
		start := cursor
		if i == ui.activeTab {
			builder.WriteString(fmt.Sprintf("[black:#8ae234]%s[-:-:-]", label))
		} else {
			builder.WriteString(fmt.Sprintf("[#729fcf]%s[-:-:-]", label))
		}
		cursor += len(label)
		hitAreas = append(hitAreas, tabHitArea{Start: start, End: cursor, Index: i})
	}
	ui.hitAreas = hitAreas
	ui.TabBar.SetText(builder.String())
}

func (ui *DashboardUI) installTabMouseHandler() {
	ui.TabBar.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action != tview.MouseLeftClick {
			return action, event
		}
		x, y := event.Position()
		rectX, rectY, _, _ := ui.TabBar.GetInnerRect()
		if y != rectY {
			return action, event
		}
		column := x - rectX

		ui.mu.RLock()
		hitAreas := append([]tabHitArea(nil), ui.hitAreas...)
		ui.mu.RUnlock()
		for _, area := range hitAreas {
			if column >= area.Start && column < area.End {
				ui.SetActiveTab(area.Index)
				return action, nil
			}
		}
		return action, event
	})
}

func RefreshInputQueue(view *tview.TextView, recent []string) {
	if len(recent) == 0 {
		view.SetText("[gray]Waiting for keyboard input...")
		return
	}
	var builder strings.Builder
	for i := len(recent) - 1; i >= 0; i-- {
		builder.WriteString(recent[i])
		if i > 0 {
			builder.WriteByte('\n')
		}
	}
	view.SetText(builder.String())
}

func RefreshBrailleGraph(graph *BrailleGraphView, samples []int) {
	graph.SetSamples(samples)
}

func RefreshTable(table *tview.Table, stats map[uint16]int) {
	type kv struct {
		Key   uint16
		Count int
	}
	var sorted []kv
	for k, v := range stats {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count == sorted[j].Count {
			return sorted[i].Key < sorted[j].Key
		}
		return sorted[i].Count > sorted[j].Count
	})

	table.Clear()

	headerColor := rgb(255, 255, 0)
	table.SetCell(0, 0, tview.NewTableCell("Key").SetTextColor(headerColor).SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("Count").SetTextColor(headerColor).SetSelectable(false))
	table.SetCell(0, 2, tview.NewTableCell("Frequency").SetTextColor(headerColor).SetSelectable(false))

	cyan := rgb(138, 226, 52)
	barLen := 20
	maxCount := 0
	for _, item := range sorted {
		if item.Count > maxCount {
			maxCount = item.Count
		}
	}
	if maxCount == 0 {
		maxCount = 1
	}

	for i, item := range sorted {
		row := i + 1
		table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("Key 0x%02X", item.Key)))
		table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", item.Count)))

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

type BrailleGraphView struct {
	*tview.Box
	mu      sync.RWMutex
	samples []int
}

func NewBrailleGraphView() *BrailleGraphView {
	box := tview.NewBox()
	box.SetBorder(true)
	box.SetTitle(" Braille Frequency Curve (30s) ")
	box.SetBorderColor(rgb(138, 226, 52))
	box.SetTitleColor(rgb(138, 226, 52))
	return &BrailleGraphView{Box: box}
}

func (g *BrailleGraphView) SetSamples(samples []int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.samples = append([]int(nil), samples...)
}

func (g *BrailleGraphView) Draw(screen tcell.Screen) {
	g.Box.DrawForSubclass(screen, g)
	x, y, width, height := g.GetInnerRect()
	if width <= 0 || height <= 0 {
		return
	}

	g.mu.RLock()
	samples := append([]int(nil), g.samples...)
	g.mu.RUnlock()

	style := tcell.StyleDefault.Foreground(rgb(138, 226, 52))
	textStyle := tcell.StyleDefault.Foreground(rgb(150, 150, 150))
	if len(samples) == 0 {
		tview.Print(screen, "Waiting for 30s bucket...", x, y+height/2, width, tview.AlignCenter, rgb(150, 150, 150))
		return
	}

	visible := width * 2
	if visible < len(samples) {
		samples = samples[len(samples)-visible:]
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

	cells := make([][]int, height*4)
	for i := range cells {
		cells[i] = make([]int, width*2)
	}

	for i, sample := range samples {
		col := i
		scaled := int(math.Round(float64(sample) / float64(maxValue) * float64(height*4)))
		if scaled < 1 && sample > 0 {
			scaled = 1
		}
		for level := 0; level < scaled && level < height*4; level++ {
			row := height*4 - 1 - level
			cells[row][col] = 1
		}
	}

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

	label := fmt.Sprintf("max %d keys / 30s", maxValue)
	tview.Print(screen, label, x, y, width, tview.AlignRight, rgb(114, 159, 207))
	_ = textStyle
}

func braillePattern(cells [][]int, cellY, cellX int) int {
	baseRow := cellY * 4
	baseCol := cellX * 2
	bits := [8]struct {
		Row int
		Col int
		Bit int
	}{
		{0, 0, 0},
		{1, 0, 1},
		{2, 0, 2},
		{0, 1, 3},
		{1, 1, 4},
		{2, 1, 5},
		{3, 0, 6},
		{3, 1, 7},
	}

	pattern := 0
	for _, bit := range bits {
		row := baseRow + bit.Row
		col := baseCol + bit.Col
		if row >= 0 && row < len(cells) && col >= 0 && col < len(cells[row]) && cells[row][col] != 0 {
			pattern |= 1 << bit.Bit
		}
	}
	return pattern
}

func rgb(r, g, b int) tcell.Color {
	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}
