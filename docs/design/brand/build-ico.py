"""CZL Browser ico 打包与验收预览。先跑 build-icons.js 生成 out/ 下的帧。

用法 (仓库根目录): python docs/design/brand/build-ico.py   (需要 Pillow)

输出:
  小尺寸符号只用于 16 / 20 档, 24 及以上一律用完整标志
  build/windows/icon.ico          exe / 安装包: 256 / 64 / 48 / 32 / 24 完整标志 + 20 / 16 小尺寸符号
  backend/internal/tray/icon.ico  托盘: 32 / 24 完整标志 + 20 / 16 小尺寸符号
  frontend/public/favicon.ico     48 / 32 完整标志 + 16 小尺寸符号
  docs/design/brand/out/preview.png     原尺寸验收预览 (48 完整标志 深 / 中灰 / 浅底; 亮暗底上 16 / 20 符号 + 24 / 32 完整标志)
  docs/design/brand/out/preview-x4.png  预览放大 4 倍 (最近邻)

ICO 手写 (PNG 帧, 大帧在前): Pillow 存 ICO 会按尺寸升序重排帧, 部分加载方取第一帧作默认图标。
"""
import struct
from pathlib import Path

from PIL import Image

BRAND = Path(__file__).resolve().parent
ROOT = BRAND.parents[2]
OUT = BRAND / "out"


def write_ico(dest: Path, frames: list[Path]) -> None:
    datas = [f.read_bytes() for f in frames]
    sizes = [Image.open(f).size for f in frames]
    header = struct.pack("<HHH", 0, 1, len(frames))
    offset = 6 + 16 * len(frames)
    entries = b""
    for (w, h), data in zip(sizes, datas):
        entries += struct.pack("<BBBBHHII", w % 256, h % 256, 0, 0, 1, 32, len(data), offset)
        offset += len(data)
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_bytes(header + entries + b"".join(datas))
    print(f"{dest.relative_to(ROOT)}: {[s[0] for s in sizes]}")


def g(s: int) -> Path:
    return OUT / f"glyph-{s}.png"


def m(s: int) -> Path:
    return OUT / f"mark-{s}.png"


write_ico(ROOT / "build/windows/icon.ico", [m(256), m(64), m(48), m(32), m(24), g(20), g(16)])
write_ico(ROOT / "backend/internal/tray/icon.ico", [m(32), m(24), g(20), g(16)])
write_ico(ROOT / "frontend/public/favicon.ico", [m(48), m(32), g(16)])

# 验收预览: 第一行 48px 完整标志 (深色壁纸 / 中灰 / 浅色, 深色底另附白色版); 第二行亮暗底上托盘各帧 16 / 20 符号 + 24 / 32 完整标志
DARK, MID, LIGHT = (30, 41, 59), (128, 128, 128), (243, 244, 246)
cell = 72
prev = Image.new("RGBA", (cell * 4, cell * 2), (255, 255, 255, 255))
row1 = [(DARK, m(48)), (MID, m(48)), (LIGHT, m(48)), (DARK, OUT / "mark-white-48.png")]
for i, (bg, f) in enumerate(row1):
    tile = Image.new("RGBA", (cell, cell), bg + (255,))
    icon = Image.open(f).convert("RGBA")
    tile.alpha_composite(icon, ((cell - icon.width) // 2, (cell - icon.height) // 2))
    prev.paste(tile, (i * cell, 0))
for j, bg in enumerate([LIGHT, DARK]):
    tile = Image.new("RGBA", (cell * 2, cell), bg + (255,))
    x = 10
    for s, f in ((16, g(16)), (20, g(20)), (24, m(24)), (32, m(32))):
        icon = Image.open(f).convert("RGBA")
        tile.alpha_composite(icon, (x, (cell - s) // 2))
        x += s + 8
    prev.paste(tile, (j * cell * 2, cell))
prev.save(OUT / "preview.png")
prev.resize((prev.width * 4, prev.height * 4), Image.NEAREST).save(OUT / "preview-x4.png")
print("preview: docs/design/brand/out/preview.png")
