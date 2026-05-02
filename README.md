# 键盘监测_btop风格

## 项目简介
这是一个基于 Golang 的键盘监测工具，采用了类似 btop 的风格，学习用。

## 功能特点
- 实时监测键盘输入
- 支持 Windows 平台的原生 API
- 直观的 UI 显示

## 使用方法

### 环境要求
- Golang 1.20 或更高版本
- Windows 操作系统

### 安装步骤
1. 克隆项目到本地：
   ```bash
   git clone https://github.com/yourusername/键盘监测_btop风格.git
   ```
2. 进入项目目录：
   ```bash
   cd 键盘监测_btop风格
   ```
3. 构建项目：
   ```bash
   go build .
   ```
4. 运行程序：
   ```bash
   ./键盘监测_btop风格.exe
   ```

## 示例
运行程序后，您将看到一个实时更新的键盘输入监测界面，类似于 btop 的风格。

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