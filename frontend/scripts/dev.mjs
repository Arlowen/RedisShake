import { createReadStream, existsSync } from 'node:fs'
import { createServer, request } from 'node:http'
import path from 'node:path'

const root = path.resolve(import.meta.dirname, '..')
const target = new URL(process.env.REDISSHAKE_API_PROXY || 'http://127.0.0.1:8080')
const types = { '.html': 'text/html; charset=utf-8', '.js': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8' }

createServer((incoming, response) => {
  if (incoming.url.startsWith('/api/') || incoming.url === '/healthz' || incoming.url === '/readyz') {
    const proxied = request({ hostname: target.hostname, port: target.port, path: incoming.url, method: incoming.method, headers: incoming.headers }, (upstream) => { response.writeHead(upstream.statusCode, upstream.headers); upstream.pipe(response) })
    incoming.pipe(proxied)
    return
  }
  const relative = incoming.url === '/' ? 'index.html' : incoming.url.replace(/^\//, '').split('?')[0]
  let file = ['app.js', 'components.js', 'lib.js', 'pages.js', 'styles.css'].includes(relative) ? path.join(root, 'src', relative) : path.join(root, relative)
  if (!existsSync(file) || !path.extname(file)) file = path.join(root, 'index.html')
  response.setHeader('Content-Type', types[path.extname(file)] || 'application/octet-stream')
  createReadStream(file).pipe(response)
}).listen(5173, '127.0.0.1', () => console.log('vanilla frontend listening on http://127.0.0.1:5173'))
