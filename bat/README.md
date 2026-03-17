# bat

## 用途

- `dev.bat`：本地开发启动
- `build.bat`：本地构建可执行文件
- `publish.bat`：打包发布安装包

## 用法

### `dev.bat`

适合日常开发。

```bat
bat\dev.bat
```

### `build.bat`

构建 `build\bin\czl-browser.exe`。

```bat
bat\build.bat
```

### `publish.bat`

生成安装包，需要先安装 NSIS。

```bat
bat\publish.bat
```

默认依赖路径：

```text
MAKENSIS_PATH -> 直接指向 makensis.exe
NSIS_PATH     -> NSIS 目录或 makensis.exe
NSIS_HOME     -> NSIS 安装目录
PATH          -> where makensis.exe
```

默认兜底目录：

```text
C:\Program Files (x86)\NSIS\makensis.exe
C:\Program Files\NSIS\makensis.exe
```

脚本使用的项目路径：

```text
输入：
- build\bin\czl-browser.exe
- publish\config.init.yaml
- bin\xray.exe
- bin\sing-box.exe
- chrome\

临时目录：
- publish\staging\

输出：
- publish\output\AntBrowser-Setup-<version>.exe
```

产物：

```text
publish\output\AntBrowser-Setup-<version>.exe
```

## 备注

- `generate-bindings.bat` 是辅助脚本，通常由 `build.bat` 调用。
