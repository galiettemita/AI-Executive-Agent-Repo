import express, { Request, Response, NextFunction } from 'express';
import crypto from 'crypto';
import { URLAllowlist } from './allowlist';
import { BrowserPool } from './browser';

const app = express();
app.use(express.json({ limit: '1mb' }));

const pool = new BrowserPool();
const allowlist = URLAllowlist.fromEnv();

const HMAC_KEY = process.env.BROWSER_MCP_HMAC_KEY ?? '';

function verifyHMAC(req: Request, res: Response, next: NextFunction): void {
  const env = process.env.BREVIO_ENV ?? '';
  if (env === 'local' || env === 'test' || env === '') { next(); return; }
  if (!HMAC_KEY) { res.status(500).json({ error: 'BROWSER_MCP_HMAC_KEY not configured' }); return; }
  const sig = req.headers['x-brevio-hmac'] as string | undefined;
  if (!sig) { res.status(401).json({ error: 'missing X-Brevio-HMAC header' }); return; }
  const body = JSON.stringify(req.body);
  const expected = crypto.createHmac('sha256', HMAC_KEY).update(body).digest('hex');
  if (sig.length !== expected.length || !crypto.timingSafeEqual(Buffer.from(sig, 'hex'), Buffer.from(expected, 'hex'))) {
    res.status(401).json({ error: 'HMAC verification failed' }); return;
  }
  next();
}

app.get('/health', (_req, res) => {
  res.json({ status: 'ok', service: 'browser-mcp', version: '1.0.0' });
});

app.post('/v1/browser/session/start', verifyHMAC, async (req, res) => {
  const { session_id, workspace_id, url, session_type } = req.body;
  if (!session_id || !workspace_id || !url) {
    res.status(400).json({ error: 'session_id, workspace_id, url are required' }); return;
  }
  const denial = allowlist.validate(url, session_type ?? 'scrape');
  if (denial) { res.status(403).json({ error: denial, code: 'URL_DENIED' }); return; }
  try {
    await pool.createSession(session_id, workspace_id);
    res.json({ session_id, status: 'active', workspace_id });
  } catch (err) {
    res.status(500).json({ error: `failed to create session: ${err}` });
  }
});

app.post('/v1/browser/navigate', verifyHMAC, async (req, res) => {
  const { session_id, url, workspace_id, session_type } = req.body;
  const denial = allowlist.validate(url, session_type ?? 'scrape');
  if (denial) { res.status(403).json({ error: denial, code: 'URL_DENIED' }); return; }
  let session = pool.getSession(session_id);
  if (!session) session = await pool.createSession(session_id, workspace_id);
  try {
    const result = await session.navigate(url);
    res.json({ session_id, result });
  } catch (err) {
    res.status(500).json({ error: `navigation failed: ${err}` });
  }
});

app.post('/v1/browser/click', verifyHMAC, async (req, res) => {
  try {
    const { session_id, workspace_id, x, y } = req.body;
    if (!session_id || x == null || y == null) {
      res.status(400).json({ error: 'session_id, x, y required' }); return;
    }
    let s = pool.getSession(session_id);
    if (!s) s = await pool.createSession(session_id, workspace_id ?? 'default');
    if (!s.page) { res.status(400).json({ error: 'no active page in session' }); return; }
    await s.page!.mouse.click(Number(x), Number(y));
    await s.page!.waitForLoadState('domcontentloaded', { timeout: 5000 }).catch(() => {});
    res.json({ ok: true });
  } catch (err: any) {
    res.status(500).json({ error: err.message ?? 'click failed' });
  }
});

app.post('/v1/browser/type', verifyHMAC, async (req, res) => {
  try {
    const { session_id, workspace_id, text } = req.body;
    if (!session_id || text == null) {
      res.status(400).json({ error: 'session_id, text required' }); return;
    }
    let s = pool.getSession(session_id);
    if (!s) s = await pool.createSession(session_id, workspace_id ?? 'default');
    if (!s.page) { res.status(400).json({ error: 'no active page in session' }); return; }
    await s.page!.keyboard.type(String(text), { delay: 50 });
    res.json({ ok: true });
  } catch (err: any) {
    res.status(500).json({ error: err.message ?? 'type failed' });
  }
});

app.post('/v1/browser/key', verifyHMAC, async (req, res) => {
  try {
    const { session_id, workspace_id, key } = req.body;
    if (!session_id || !key) {
      res.status(400).json({ error: 'session_id, key required' }); return;
    }
    let s = pool.getSession(session_id);
    if (!s) s = await pool.createSession(session_id, workspace_id ?? 'default');
    if (!s.page) { res.status(400).json({ error: 'no active page in session' }); return; }
    await s.page!.keyboard.press(String(key));
    res.json({ ok: true });
  } catch (err: any) {
    res.status(500).json({ error: err.message ?? 'key failed' });
  }
});

app.post('/v1/browser/scroll', verifyHMAC, async (req, res) => {
  try {
    const { session_id, workspace_id, direction, amount } = req.body;
    if (!session_id || !direction) {
      res.status(400).json({ error: 'session_id, direction required' }); return;
    }
    let s = pool.getSession(session_id);
    if (!s) s = await pool.createSession(session_id, workspace_id ?? 'default');
    if (!s.page) { res.status(400).json({ error: 'no active page in session' }); return; }
    const delta = (direction === 'down' ? 1 : -1) * Number(amount || 3) * 100;
    await s.page!.mouse.wheel(0, delta);
    res.json({ ok: true });
  } catch (err: any) {
    res.status(500).json({ error: err.message ?? 'scroll failed' });
  }
});

app.post('/v1/browser/dom', verifyHMAC, async (req, res) => {
  try {
    const { session_id, workspace_id } = req.body;
    if (!session_id) { res.status(400).json({ error: 'session_id required' }); return; }
    let s = pool.getSession(session_id);
    if (!s) s = await pool.createSession(session_id, workspace_id ?? 'default');
    if (!s.page) { res.status(400).json({ error: 'no active page in session' }); return; }
    const dom = await s.page!.evaluate(() => {
      const els: string[] = [];
      document.querySelectorAll('button,a,input,select,textarea,[role=button],[role=link]')
        .forEach(el => {
          const t = (el as HTMLElement).innerText?.trim() || '';
          const r = el.getBoundingClientRect();
          if (r.width > 0 && r.height > 0 && t)
            els.push(`${el.tagName.toLowerCase()}[${Math.round(r.x)},${Math.round(r.y)}]: ${t.slice(0, 60)}`);
        });
      return els.join('\n');
    });
    res.json({ dom });
  } catch (err: any) {
    res.status(500).json({ error: err.message ?? 'dom failed' });
  }
});

app.post('/v1/browser/scrape', verifyHMAC, async (req, res) => {
  const { session_id, url, workspace_id, selectors } = req.body;
  const denial = allowlist.validate(url, 'scrape');
  if (denial) { res.status(403).json({ error: denial, code: 'URL_DENIED' }); return; }
  let session = pool.getSession(session_id);
  if (!session) session = await pool.createSession(session_id, workspace_id);
  try {
    const result = await session.scrape(url, selectors);
    res.json({ session_id, result });
  } catch (err) {
    res.status(500).json({ error: `scrape failed: ${err}` });
  }
});

app.post('/v1/browser/form-fill', verifyHMAC, async (req, res) => {
  const { session_id, url, workspace_id, fields, submit_selector } = req.body;
  const denial = allowlist.validate(url, 'form_fill');
  if (denial) { res.status(403).json({ error: denial, code: 'FORM_FILL_DENIED' }); return; }
  if (!fields || Object.keys(fields).length === 0) {
    res.status(400).json({ error: 'fields is required and must be non-empty' }); return;
  }
  let session = pool.getSession(session_id);
  if (!session) session = await pool.createSession(session_id, workspace_id);
  try {
    const result = await session.formFill(url, fields, submit_selector);
    res.json({ session_id, result });
  } catch (err) {
    res.status(500).json({ error: `form fill failed: ${err}` });
  }
});

app.post('/v1/browser/screenshot', verifyHMAC, async (req, res) => {
  const { session_id } = req.body;
  const session = pool.getSession(session_id);
  if (!session) { res.status(404).json({ error: `session ${session_id} not found` }); return; }
  try {
    const result = await session.screenshot();
    res.json({ session_id, result });
  } catch (err) {
    res.status(500).json({ error: `screenshot failed: ${err}` });
  }
});

app.post('/v1/browser/session/close', verifyHMAC, async (req, res) => {
  const { session_id } = req.body;
  await pool.closeSession(session_id);
  res.json({ session_id, status: 'closed' });
});

async function shutdown(): Promise<void> {
  await pool.shutdown();
  process.exit(0);
}
process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

const PORT = parseInt(process.env.BROWSER_MCP_PORT ?? '7788', 10);
app.listen(PORT, '0.0.0.0', () => {
  console.log(JSON.stringify({
    level: 'info',
    message: 'browser-mcp started',
    port: PORT,
    allowlist_entries: allowlist.getEntries().length,
    env: process.env.BREVIO_ENV ?? 'local',
  }));
});

export default app;
