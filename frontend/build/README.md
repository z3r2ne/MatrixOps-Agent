# Electron 应用图标

此目录用于存放 Electron 应用的图标文件。

**更新流程**：修改 `icon.svg` 后，在 `frontend` 目录执行：

```bash
npm run electron:icons
```

会生成/覆盖 `icon.png`、`icon.icns`、`icon.ico`（macOS 上会先用 `sips` 导出 PNG）。然后再执行 `npm run electron:build` 等打包命令。

## 所需图标文件

### macOS
- `icon.icns` - macOS 应用图标（512x512px 或更大）

### Windows
- `icon.ico` - Windows 应用图标（256x256px 或更大）

### Linux
- `icon.png` - Linux 应用图标（512x512px 或更大，PNG 格式）

## 生成图标

### 方法 1：在线工具
1. 访问 https://www.icoconverter.com/ 或类似工具
2. 上传你的源图标（建议 1024x1024px PNG）
3. 生成所需格式的图标文件
4. 将文件放入此目录

### 方法 2：使用 electron-icon-builder

```bash
# 安装工具
npm install -g electron-icon-builder

# 准备一个 1024x1024 的 PNG 图标（如 icon.png）
# 生成所有平台的图标
electron-icon-builder --input=./icon.png --output=./build --flatten
```

### 方法 3：使用 ImageMagick

**生成 ICO（Windows）:**
```bash
convert icon.png -resize 256x256 icon.ico
```

**生成 ICNS（macOS）:**
```bash
# 创建 iconset 目录
mkdir icon.iconset

# 生成不同尺寸
for size in 16 32 128 256 512; do
  convert icon.png -resize ${size}x${size} icon.iconset/icon_${size}x${size}.png
done

# 生成 @2x 版本
for size in 32 64 256 512; do
  convert icon.png -resize ${size}x${size} icon.iconset/icon_$((size/2))x$((size/2))@2x.png
done

# 生成 icns
iconutil -c icns icon.iconset
```

## 图标设计建议

1. **尺寸**: 至少 1024x1024px
2. **格式**: 源文件使用 PNG 格式，带透明背景
3. **风格**: 简洁、清晰，在小尺寸下也能识别
4. **颜色**: 避免过于复杂的渐变
5. **测试**: 在不同尺寸和背景下测试显示效果

## 临时方案

在正式图标准备好之前，你可以使用 Electron 的默认图标，应用会正常运行。

## 注意事项

- 图标文件应该使用无损压缩
- macOS 的 ICNS 需要包含多种尺寸（16、32、64、128、256、512、1024）
- Windows 的 ICO 通常包含 16、32、48、64、128、256 尺寸
- 构建时会自动使用这些图标文件
