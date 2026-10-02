import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';

const page = new URL('./index.html', import.meta.url);
const port = Number(process.env.PORT || 8091);
const server = createServer(async (req, res) => {
  if (req.method !== 'GET' || new URL(req.url, 'http://localhost').pathname !== '/') {
    res.writeHead(404).end('Not found');
    return;
  }
  try {
    const body = await readFile(page);
    res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'no-store' });
    res.end(body);
  } catch (err) {
    console.error(err);
    res.writeHead(500).end('Unable to read simulation report');
  }
});
server.listen(port, '127.0.0.1', () => console.log(`Simulation report: http://localhost:${port}`));
