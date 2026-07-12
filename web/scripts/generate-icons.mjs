// Rasterize web/public/icon.svg into the PWA icon set.
//
// Usage: npm run gen:icons  (from web/)
//
// The generated PNGs are committed to the repo — CI just runs `npm ci &&
// vite build`, so the rasters must exist ahead of time. Re-run this script
// whenever icon.svg changes.
import { fileURLToPath } from 'node:url'
import path from 'node:path'
import sharp from 'sharp'

const publicDir = path.join(path.dirname(fileURLToPath(import.meta.url)), '..', 'public')
const src = path.join(publicDir, 'icon.svg')

// Matches the rounded-square background fill in icon.svg.
const BACKGROUND = '#8b8ff5'

/** Render icon.svg at the given square size (transparent rounded corners kept). */
function renderIcon(size) {
  return sharp(src, { density: (72 * size) / 512 })
    .resize(size, size)
    .png()
    .toBuffer()
}

async function main() {
  // Standard icons: the SVG as-is (rounded square, transparent corners).
  await renderIcon(192).then((buf) => sharp(buf).toFile(path.join(publicDir, 'pwa-192x192.png')))
  await renderIcon(512).then((buf) => sharp(buf).toFile(path.join(publicDir, 'pwa-512x512.png')))

  // Maskable icon: same art scaled to 60% and centered on a full-bleed
  // background square. The icon's own rounded corners blend into the matching
  // background, leaving the barbell glyph comfortably inside the 80% "safe
  // zone" circle (20% padding on every side).
  const inner = Math.round(512 * 0.6) // 307px art, ~102px (20%) padding per side
  const maskableArt = await renderIcon(inner)
  await sharp({
    create: { width: 512, height: 512, channels: 4, background: BACKGROUND },
  })
    .composite([{ input: maskableArt, gravity: 'centre' }])
    .png()
    .toFile(path.join(publicDir, 'pwa-maskable-512x512.png'))

  // Apple touch icon: 180x180, no transparency — iOS applies its own corner
  // mask, so flatten the rounded corners onto the background color.
  await renderIcon(180).then((buf) =>
    sharp(buf)
      .flatten({ background: BACKGROUND })
      .toFile(path.join(publicDir, 'apple-touch-icon.png')),
  )

  // Favicon raster fallback (the SVG favicon covers modern browsers).
  await renderIcon(32).then((buf) => sharp(buf).toFile(path.join(publicDir, 'favicon-32.png')))

  console.log('Generated: pwa-192x192.png, pwa-512x512.png, pwa-maskable-512x512.png, apple-touch-icon.png, favicon-32.png')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
