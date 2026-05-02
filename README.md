# 键盘监测_btop风格

## 项目简介
这是一个基于 Golang 的键盘监测工具，采用了类似 btop 的风格，学习用。

## 功能特点
- 实时监测键盘输入
- 支持 Windows 平台的原生 API
- 直观的 UI 显示

## 技术说明
- 本项目使用 vibe coding 作为开发方式
- UI 基于 [tview](https://github.com/rivo/tview) 库
- 终端渲染基于 [tcell](https://github.com/gdamore/tcell) 库

## 使用方法

### 环境要求
- Golang 1.20 或更高版本
- Windows 操作系统



## 文件结构
```
键盘监测_btop风格/
├── go.mod                # Go 模块文件
├── main.go               # 主程序入口
├── internal/
│   ├── display/
│   │   └── UI.go         # UI 相关代码
│   ├── keyboard/
│       ├── RawInput_study.go  # 键盘输入处理
│       ├── RawInput.txt       # 键盘输入示例数据
│       └── WindowsAPI.go      # Windows API 封装
```

## 贡献
欢迎提交 Issue 或 Pull Request 来改进本项目。

## 开源协议
本项目采用 [MIT License](LICENSE) 开源协议。

## 作者
- **fyuo863** - [GitHub Profile](https://github.com/fyuo863)

---