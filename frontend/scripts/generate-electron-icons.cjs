/**
 * 从 build/icon-source.png 或 build/icon.svg 生成 Electron / electron-builder 所需的位图图标。
 * - 若存在 icon-source.png：优先将其作为 824×824 主体源图，整体放大后居中生成圆角卡片，再放入 1024×1024 透明画布 build/icon.png
 * - 若不存在 icon-source.png：回退到 icon.svg；macOS 上会先用 sips 导出 PNG
 * - 最终统一生成/覆盖 build/icon.png、build/icon.icns、build/icon.ico
 */
const { execFileSync, execSync } = require("child_process")
const fs = require("fs")
const os = require("os")
const path = require("path")

const root = path.join(__dirname, "..")
const buildDir = path.join(root, "build")
const sourcePng = path.join(buildDir, "icon-source.png")
const svg = path.join(buildDir, "icon.svg")
const pngOut = path.join(buildDir, "icon.png")
const artworkSize = 824
const canvasSize = 1024
const cornerRadius = 184
const sourceScale = 1.14

function run(cmd, cwd = root) {
  execSync(cmd, { stdio: "inherit", cwd, shell: true })
}

function createPaddedPng(input, output) {
  const script = `
import sys
from PIL import Image, ImageDraw, ImageChops

source_path, output_path, artwork_size, canvas_size = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
radius = int(sys.argv[5])
scale = float(sys.argv[6])

source = Image.open(source_path).convert("RGBA")
if source.size != (artwork_size, artwork_size):
    source = source.resize((artwork_size, artwork_size), Image.LANCZOS)

scaled_size = max(artwork_size, round(artwork_size * scale))
source = source.resize((scaled_size, scaled_size), Image.LANCZOS)

card = Image.new("RGBA", (artworkSize := artwork_size, artworkSize), (255, 255, 255, 0))
card_offset = ((artwork_size - scaled_size) // 2, (artwork_size - scaled_size) // 2)
card.alpha_composite(source, card_offset)

mask = Image.new("L", (artwork_size, artwork_size), 0)
draw = ImageDraw.Draw(mask)
draw.rounded_rectangle((0, 0, artwork_size, artwork_size), radius=radius, fill=255)
card_alpha = card.getchannel("A")
card.putalpha(ImageChops.multiply(card_alpha, mask))

canvas = Image.new("RGBA", (canvas_size, canvas_size), (0, 0, 0, 0))
offset = ((canvas_size - artwork_size) // 2, (canvas_size - artwork_size) // 2)
canvas.alpha_composite(card, offset)
canvas.save(output_path)
`

  try {
    execFileSync("python3", ["-c", script, input, output, String(artworkSize), String(canvasSize), String(cornerRadius), String(sourceScale)], {
      stdio: "inherit",
      cwd: root,
    })
    return true
  } catch (error) {
    console.warn(
      "[electron:icons] 无法生成圆角透明边距 PNG，已回退为直接复制 icon-source.png。请确认 python3 和 Pillow 可用。",
    )
    fs.copyFileSync(input, output)
    return false
  }
}

let iconInput = ""

if (fs.existsSync(sourcePng)) {
  createPaddedPng(sourcePng, pngOut)
  iconInput = pngOut
} else if (fs.existsSync(svg)) {
  if (os.platform() === "darwin") {
    run(`sips -s format png "${svg}" --out "${pngOut}"`)
    iconInput = pngOut
  } else {
    iconInput = svg
    console.warn(
      "[electron:icons] 当前未提供 build/icon-source.png，继续使用 build/icon.svg 生成图标。",
    )
  }
} else {
  console.error("Missing icon source. Expected one of:", sourcePng, "or", svg)
  process.exit(1)
}

run(
  `npx --yes icon-gen -i "${iconInput}" -o "${buildDir}" --ico --ico-name icon --icns --icns-name icon -r`,
)

console.log("已生成 build/icon.png、build/icon.icns、build/icon.ico")
