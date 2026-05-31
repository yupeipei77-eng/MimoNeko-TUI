package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	binaryName  = "mimoneko"
	installDir  = ".mimoneko/bin"
	pathComment = "# MimoNeko"
)

func main() {
	fmt.Println()
	fmt.Println("=====================================")
	fmt.Println("  MimoNeko 安装程序")
	fmt.Println("=====================================")
	fmt.Println()

	// 获取可执行文件路径
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	// 获取用户主目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	// 计算安装目录
	targetDir := filepath.Join(homeDir, installDir)
	targetPath := filepath.Join(targetDir, binaryName)
	if runtime.GOOS == "windows" {
		targetPath += ".exe"
	}

	// 创建安装目录
	fmt.Printf("[1/3] 创建安装目录: %s\n", targetDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	// 复制文件
	fmt.Printf("[2/3] 复制 %s\n", binaryName)
	if err := copyFile(exePath, targetPath); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  已安装到: %s\n", targetPath)

	// 配置 PATH
	fmt.Println("[3/3] 配置 PATH 环境变量")
	if err := configurePath(homeDir, targetDir); err != nil {
		fmt.Printf("警告: %v\n", err)
		printManualPathInstructions(targetDir)
	} else {
		fmt.Println("  已配置 PATH")
	}

	fmt.Println()
	fmt.Println("=====================================")
	fmt.Println("  安装完成！")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("请重新打开终端，然后运行:")
	fmt.Printf("  %s --help\n", binaryName)
	fmt.Println()
	fmt.Println("配置 API Key:")
	fmt.Println("  export MIMO_API_KEY=\"your-api-key\"")
	fmt.Println()
}

func copyFile(src, dst string) error {
	// 读取源文件
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// 写入目标文件
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return err
	}

	return nil
}

func configurePath(homeDir, installDir string) error {
	// 根据操作系统和 shell 选择配置文件
	configFile := getShellConfig(homeDir)
	if configFile == "" {
		return fmt.Errorf("无法确定 shell 配置文件")
	}

	// 读取配置文件内容
	content, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建新文件
			return createPathConfig(configFile, installDir)
		}
		return err
	}

	// 检查是否已配置
	if strings.Contains(string(content), installDir) {
		fmt.Println("  PATH 已包含安装目录")
		return nil
	}

	// 追加配置
	return appendPathConfig(configFile, installDir)
}

func getShellConfig(homeDir string) string {
	switch runtime.GOOS {
	case "windows":
		// Windows: 添加到系统 PATH
		return "" // Windows 需要特殊处理
	case "darwin":
		// macOS: 优先 .zshrc
		configs := []string{".zshrc", ".bash_profile", ".profile"}
		for _, config := range configs {
			path := filepath.Join(homeDir, config)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		return filepath.Join(homeDir, ".zshrc")
	default:
		// Linux: 优先 .bashrc
		configs := []string{".bashrc", ".bash_profile", ".profile", ".zshrc"}
		for _, config := range configs {
			path := filepath.Join(homeDir, config)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		return filepath.Join(homeDir, ".profile")
	}
}

func createPathConfig(configFile, installDir string) error {
	content := fmt.Sprintf("\n%s\nexport PATH=\"%s:$PATH\"\n", pathComment, installDir)
	return os.WriteFile(configFile, []byte(content), 0644)
}

func appendPathConfig(configFile, installDir string) error {
	// 读取现有内容
	content, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	// 追加新配置
	newContent := string(content) + fmt.Sprintf("\n%s\nexport PATH=\"%s:$PATH\"\n", pathComment, installDir)
	return os.WriteFile(configFile, []byte(newContent), 0644)
}

func printManualPathInstructions(installDir string) {
	fmt.Println("  请手动添加以下内容到你的 shell 配置文件:")
	fmt.Println()

	switch runtime.GOOS {
	case "windows":
		fmt.Printf("  set PATH=%%PATH%%;%s\n", installDir)
	default:
		fmt.Printf("  export PATH=\"%s:$PATH\"\n", installDir)
	}
}

// addToWindowsPath 添加到 Windows 系统 PATH
func addToWindowsPath(installDir string) error {
	// 使用 PowerShell 添加到用户 PATH
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ';%s', 'User')", installDir))
	return cmd.Run()
}
