import platform
import sys

def get_system_version():
    """获取系统版本信息"""
    try:
        system_info = {
            "操作系统": platform.system(),
            "版本": platform.release(),
            "详细版本": platform.version(),
            "架构": platform.architecture()[0],
            "机器类型": platform.machine(),
            "处理器": platform.processor(),
            "Python版本": platform.python_version()
        }
        
        # 针对不同操作系统补充信息
        if platform.system() == "Windows":
            try:
                import winreg
                key = winreg.OpenKey(winreg.HKEY_LOCAL_MACHINE, r"SOFTWARE\Microsoft\Windows NT\CurrentVersion")
                system_info["Windows内部版本"] = winreg.QueryValueEx(key, "CurrentBuild")[0]
                winreg.CloseKey(key)
            except:
                pass
        elif platform.system() == "Linux":
            try:
                with open("/etc/os-release", "r") as f:
                    lines = f.readlines()
                    for line in lines:
                        if line.startswith("PRETTY_NAME="):
                            system_info["发行版"] = line.split("=")[1].strip().strip('"')
                            break
            except:
                pass
        elif platform.system() == "Darwin":
            system_info["macOS版本"] = platform.mac_ver()[0]
        
        return system_info
    except Exception as e:
        return {"错误": f"获取系统信息失败: {str(e)}"}

def display_system_version():
    """显示系统版本信息"""
    print("=" * 50)
    print("系统版本信息")
    print("=" * 50)
    
    system_info = get_system_version()
    
    if "错误" in system_info:
        print(f"错误: {system_info['错误']}")
        return
    
    for key, value in system_info.items():
        print(f"{key}: {value}")
    
    print("=" * 50)
    print(f"系统: {system_info.get('操作系统', '未知')} {system_info.get('版本', '')}")
    print(f"架构: {system_info.get('架构', '未知')}")
    print(f"Python: {system_info.get('Python版本', '未知')}")

if __name__ == "__main__":
    display_system_version()
