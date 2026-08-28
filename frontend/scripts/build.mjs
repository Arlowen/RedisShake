import { cp, mkdir, readdir, rm } from 'node:fs/promises'
import path from 'node:path'

const root = path.resolve(import.meta.dirname, '..')
const dist = path.join(root, 'dist')
await rm(dist, { recursive: true, force: true })
await mkdir(dist, { recursive: true })
await cp(path.join(root, 'index.html'), path.join(dist, 'index.html'))
for (const entry of await readdir(path.join(root, 'src'))) {
  if (entry.endsWith('.js') || entry.endsWith('.css')) await cp(path.join(root, 'src', entry), path.join(dist, entry))
}
console.log('pure HTML/CSS/JavaScript assets built')
