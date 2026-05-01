package display

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BuildDashboard 创建整个 tview 界面树
// 返回根节点（用于 app.SetRoot）和 statsTable（供外部刷新数据）
func BuildDashboard() (tview.Primitive, *tview.Table) {
	// ═══ 主表格：按键排行榜 ═══
	statsTable := tview.NewTable()
	statsTable.SetBorder(true)
	statsTable.SetTitle(" Key Counter Pro (btop-style) ")
	statsTable.SetBorderColor(rgb(230, 50, 100)) // 粉红边框
	statsTable.SetTitleColor(rgb(230, 50, 100))  // 粉红标题
	statsTable.SetSelectable(false, false)       // 只能按行选中

	// ═══ 底部状态栏 ═══
	footer := tview.NewTextView()
	footer.SetText("Press 'q' or Ctrl+C to quit | Monitoring Global Input...")
	footer.SetTextColor(rgb(150, 150, 150)) // 灰色

	// ═══ 布局：表格填满上方，底部留一行 ═══
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(statsTable, 0, 1, true) // 比例 1，占据剩余空间
	flex.AddItem(footer, 1, 0, false)    // 固定 1 行高

	return flex, statsTable
}

// RefreshTable 用最新统计数据刷新表格内容
// stats 的 key 是虚拟键码，value 是按下次数
func RefreshTable(table *tview.Table, stats map[uint16]int) {
	// --- 排序：按次数从高到低 ---
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
			// 次数相同，按 Key 升序排列（小的在前）
			return sorted[i].Key < sorted[j].Key
		}
		return sorted[i].Count > sorted[j].Count // 降序：次数多的在前面
	})

	table.Clear()

	// --- 表头 ---
	headerColor := rgb(255, 255, 0) // 黄色
	table.SetCell(0, 0, tview.NewTableCell("Key").SetTextColor(headerColor).SetSelectable(false))
	table.SetCell(0, 1, tview.NewTableCell("Count").SetTextColor(headerColor).SetSelectable(false))
	table.SetCell(0, 2, tview.NewTableCell("Frequency").SetTextColor(headerColor).SetSelectable(false))

	// --- 数据行 ---
	cyan := rgb(138, 226, 52)
	barLen := 20 // 进度条总格数

	for i, item := range sorted {
		row := i + 1

		// 键名（如 "Key 0x41"）
		table.SetCell(row, 0, tview.NewTableCell(fmt.Sprintf("Key 0x%02X", item.Key)))

		// 次数
		table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%d", item.Count)))

		// 进度条：用 █ 填充 + 空格留白
		barPercent := item.Count
		if barPercent > 100 {
			barPercent = 100
		}
		fillLen := barPercent * barLen / 100
		bar := fmt.Sprintf("[%s%s] %d",
			strings.Repeat("█", fillLen),
			strings.Repeat(" ", barLen-fillLen),
			barPercent,
		)
		table.SetCell(row, 2, tview.NewTableCell(bar).SetTextColor(cyan))
	}
}

// rgb 是本包内唯一的 tcell 颜色入口，其余代码只调用 rgb() 而不直接接触 tcell
func rgb(r, g, b int) tcell.Color {
	return tcell.NewRGBColor(int32(r), int32(g), int32(b))
}
