// CZL Browser 图标出图脚本: 读 mark.svg (完整标志) / glyph.svg (小尺寸符号), 生成各尺寸 PNG 与前端 SVG。
// mark.svg / glyph.svg 从 czl-skill references/product-logo/{marks,glyphs}/czlbrowser.svg 拷贝, 不在本仓库改造型。
//
// 用法 (仓库根目录):
//   npm i --no-save --prefix <临时目录> @resvg/resvg-js@2.6.2
//   NODE_PATH=<临时目录>/node_modules node docs/design/brand/build-icons.js
//   python docs/design/brand/build-ico.py
//
// 输出:
//   build/appicon.png                         1024 透明底完整标志 (长边 92%)
//   frontend/public/icon.svg                  favicon, 小尺寸符号
//   frontend/src/resources/images/logo.svg    完整标志 (渐变)
//   frontend/src/resources/images/logo-white.svg  完整标志 (白色, 暗色主题)
//   docs/design/brand/out/                    ico 帧与预览用 PNG (不入库)
const fs = require('fs')
const path = require('path')
const { Resvg } = require('@resvg/resvg-js')

const root = path.resolve(__dirname, '../../..')
const brand = __dirname
const out = path.join(brand, 'out')
fs.mkdirSync(out, { recursive: true })

const markSrc = fs.readFileSync(path.join(brand, 'mark.svg'), 'utf8')
const glyphSrc = fs.readFileSync(path.join(brand, 'glyph.svg'), 'utf8')

// 把 SVG 的视窗改成以图形为中心的正方形, 图形长边占 ratio
function square(svg, ratio) {
  const m = svg.match(/viewBox="([\d.\s-]+)"/)
  const [x, y, w, h] = m[1].trim().split(/\s+/).map(Number)
  const side = Math.max(w, h) / ratio
  const nx = x + w / 2 - side / 2
  const ny = y + h / 2 - side / 2
  return svg.replace(m[0], `viewBox="${nx.toFixed(2)} ${ny.toFixed(2)} ${side.toFixed(2)} ${side.toFixed(2)}"`)
}

function png(svg, size, file) {
  const data = new Resvg(svg, { fitTo: { mode: 'width', value: size } }).render().asPng()
  fs.writeFileSync(file, data)
}

const markSq = square(markSrc, 0.92)
const glyphSq = square(glyphSrc, 0.97)
const white = (svg) => svg.replace(/fill="url\(#g\)"/g, 'fill="#FFFFFF"')

// 应用图标
png(markSq, 1024, path.join(root, 'build/appicon.png'))

// 前端 SVG (侧栏与登录页都 ≥24px, 用完整标志; 小尺寸符号只用于 16 / 20 档)
const img = path.join(root, 'frontend/src/resources/images')
fs.writeFileSync(path.join(img, 'logo.svg'), markSrc)
fs.writeFileSync(path.join(img, 'logo-white.svg'), white(markSrc))
fs.writeFileSync(path.join(root, 'frontend/public/icon.svg'), glyphSq)

// ico 帧与预览
for (const s of [16, 20]) png(glyphSq, s, path.join(out, `glyph-${s}.png`))
for (const s of [24, 32, 48, 64, 256]) png(markSq, s, path.join(out, `mark-${s}.png`))
png(white(markSq), 48, path.join(out, 'mark-white-48.png'))
console.log('done')
