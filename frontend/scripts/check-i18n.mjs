import fs from 'node:fs'
import path from 'node:path'

const root = path.resolve(process.cwd(), 'src')
const localeFile = path.join(root, 'locales', 'zh-CN.js')
const mod = await import(localeFile)
const messages = mod.default

const flat = new Set()
const walk = (obj, prefix = '') => {
  for (const [key, value] of Object.entries(obj)) {
    const full = prefix ? `${prefix}.${key}` : key
    if (value && typeof value === 'object') walk(value, full)
    else flat.add(full)
  }
}
walk(messages)

const files = []
const collect = (dir) => {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) collect(full)
    else if (/\.(vue|js)$/.test(entry.name) && !full.includes(`${path.sep}locales${path.sep}`)) files.push(full)
  }
}
collect(root)

const missing = new Map()
for (const file of files) {
  const text = fs.readFileSync(file, 'utf8')
  const regex = /\bt\(\s*['"`]([a-zA-Z0-9_.]+)['"`]/g
  let m
  while ((m = regex.exec(text))) {
    const key = m[1]
    if (!flat.has(key)) {
      const rel = path.relative(root, file)
      if (!missing.has(rel)) missing.set(rel, new Set())
      missing.get(rel).add(key)
    }
  }
}

if (!missing.size) {
  console.log(`OK: all t() keys exist in zh-CN (${flat.size} keys)`) 
} else {
  console.log('MISSING KEYS:')
  for (const [file, keys] of missing) console.log(`\n${file}:\n  ` + [...keys].join('\n  '))
  process.exitCode = 1
}
