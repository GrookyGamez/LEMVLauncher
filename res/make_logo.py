#!/usr/bin/env python3
"""Render the LEMV mark as high-res PNGs.

The mark is the launcher's app icon: an emerald rounded square with a
rounded-square hole punched through the middle. Edges are antialiased with a
signed-distance field so it stays crisp when Discord scales it down.
"""
import struct
import zlib
import numpy as np

ACCENT = (61, 214, 140)   # #3DD68C, the launcher accent
DARK = (26, 27, 30)       # #1A1B1E, the launcher background


def rounded_rect_alpha(size, cx, cy, half_w, half_h, radius):
    """Coverage mask (0..1) of a rounded rectangle, antialiased."""
    ax = np.arange(size, dtype=np.float64) + 0.5
    X, Y = np.meshgrid(ax, ax)
    dx = np.abs(X - cx) - (half_w - radius)
    dy = np.abs(Y - cy) - (half_h - radius)
    dx = np.clip(dx, 0, None)
    dy = np.clip(dy, 0, None)
    dist = np.sqrt(dx * dx + dy * dy) - radius
    return np.clip(0.5 - dist, 0.0, 1.0)


def make_mark(size, mark_span, bg=None):
    """RGBA array of the mark centred on the canvas.

    mark_span: width of the mark in pixels.
    bg: optional opaque background colour; None leaves it transparent.
    """
    c = size / 2.0
    half = mark_span / 2.0
    outer = rounded_rect_alpha(size, c, c, half, half, mark_span * 0.26)
    hole_half = mark_span * 0.20
    hole = rounded_rect_alpha(size, c, c, hole_half, hole_half, mark_span * 0.075)
    ring = outer * (1.0 - hole)

    rgb = np.zeros((size, size, 3), dtype=np.float64)
    alpha = np.zeros((size, size), dtype=np.float64)

    if bg is not None:
        rgb[:] = bg
        alpha[:] = 1.0

    for i, ch in enumerate(ACCENT):
        rgb[:, :, i] = rgb[:, :, i] * (1 - ring) + ch * ring
    alpha = np.clip(alpha + ring, 0.0, 1.0)

    out = np.zeros((size, size, 4), dtype=np.uint8)
    out[:, :, :3] = np.round(rgb).astype(np.uint8)
    out[:, :, 3] = np.round(alpha * 255).astype(np.uint8)
    return out


def write_png(path, arr):
    size = arr.shape[0]
    raw = b"".join(b"\x00" + arr[y].tobytes() for y in range(size))

    def chunk(tag, data):
        body = tag + data
        return struct.pack(">I", len(data)) + body + struct.pack(">I", zlib.crc32(body) & 0xFFFFFFFF)

    png = (b"\x89PNG\r\n\x1a\n"
           + chunk(b"IHDR", struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0))
           + chunk(b"IDAT", zlib.compress(raw, 9))
           + chunk(b"IEND", b""))
    with open(path, "wb") as fh:
        fh.write(png)
    return len(png)


if __name__ == "__main__":
    S = 1024
    # Discord crops app icons to a circle, so keep the mark inside the
    # inscribed square (~70% of the canvas) and sit it on the dark background.
    disc = make_mark(S, mark_span=int(S * 0.62), bg=DARK)
    n1 = write_png("/mnt/user-data/outputs/lemv-logo-discord.png", disc)

    # Full-bleed transparent version: exactly the app icon geometry.
    trans = make_mark(S, mark_span=int(S * 0.88), bg=None)
    n2 = write_png("/mnt/user-data/outputs/lemv-logo-transparent.png", trans)

    print(f"lemv-logo-discord.png     {S}x{S}  {n1:,} bytes")
    print(f"lemv-logo-transparent.png {S}x{S}  {n2:,} bytes")
