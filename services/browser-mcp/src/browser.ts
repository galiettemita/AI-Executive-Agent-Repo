import { chromium, Browser, BrowserContext, Page } from 'playwright';

export interface NavigateResult {
  url: string;
  title: string;
  statusCode: number;
  bodyText: string;
}

export interface FormFillResult {
  success: boolean;
  submissionId: string;
  fieldsFilled: number;
}

export interface ScrapeResult {
  url: string;
  title: string;
  bodyText: string;
  links: string[];
  extractedData: Record<string, string>;
}

export interface ScreenshotResult {
  dataBase64: string;
  width: number;
  height: number;
  format: 'png';
}

export class BrowserSession {
  private context: BrowserContext;
  page: Page | null = null;
  readonly sessionId: string;
  readonly workspaceId: string;

  constructor(context: BrowserContext, sessionId: string, workspaceId: string) {
    this.context = context;
    this.sessionId = sessionId;
    this.workspaceId = workspaceId;
  }

  async navigate(url: string): Promise<NavigateResult> {
    this.page = await this.context.newPage();
    await this.page.route('**/*.{png,jpg,gif,webp,css,woff,woff2}', (route) => route.abort());

    const response = await this.page.goto(url, {
      waitUntil: 'domcontentloaded',
      timeout: 15000,
    });

    const title = await this.page.title();
    const bodyText = await this.page.evaluate(() => document.body?.innerText ?? '');

    return {
      url: this.page.url(),
      title,
      statusCode: response?.status() ?? 200,
      bodyText: bodyText.slice(0, 10000),
    };
  }

  async scrape(url: string, selectors?: Record<string, string>): Promise<ScrapeResult> {
    const navResult = await this.navigate(url);

    const links = await this.page!.evaluate(() => {
      return Array.from(document.querySelectorAll('a[href]'))
        .map((a) => (a as HTMLAnchorElement).href)
        .filter((href) => href.startsWith('https://'))
        .slice(0, 50);
    });

    const extractedData: Record<string, string> = {};
    if (selectors) {
      for (const [key, selector] of Object.entries(selectors)) {
        try {
          const el = await this.page!.$(selector);
          if (el) extractedData[key] = (await el.textContent()) ?? '';
        } catch { /* selector not found */ }
      }
    }

    return { url: navResult.url, title: navResult.title, bodyText: navResult.bodyText, links, extractedData };
  }

  async formFill(url: string, fields: Record<string, string>, submitSelector?: string): Promise<FormFillResult> {
    await this.navigate(url);
    let fieldsFilled = 0;

    for (const [selector, value] of Object.entries(fields)) {
      try {
        await this.page!.fill(selector, value, { timeout: 5000 });
        fieldsFilled++;
      } catch { /* field not found */ }
    }

    if (submitSelector) {
      await this.page!.click(submitSelector, { timeout: 5000 });
      await this.page!.waitForLoadState('domcontentloaded', { timeout: 10000 });
    }

    return { success: fieldsFilled > 0, submissionId: `form-${this.sessionId}-${Date.now()}`, fieldsFilled };
  }

  async screenshot(): Promise<ScreenshotResult> {
    if (!this.page) throw new Error('No page loaded — navigate first');
    const buffer = await this.page.screenshot({ type: 'png', fullPage: false });
    const vp = this.page.viewportSize();
    return { dataBase64: buffer.toString('base64'), width: vp?.width ?? 1280, height: vp?.height ?? 720, format: 'png' };
  }

  async close(): Promise<void> {
    try { if (this.page && !this.page.isClosed()) await this.page.close(); } catch { /* ignore */ }
    try { await this.context.close(); } catch { /* ignore */ }
  }
}

export class BrowserPool {
  private browser: Browser | null = null;
  private sessions: Map<string, BrowserSession> = new Map();
  private starting = false;

  async ensureBrowser(): Promise<Browser> {
    if (this.browser && this.browser.isConnected()) return this.browser;
    if (this.starting) {
      await new Promise((resolve) => setTimeout(resolve, 500));
      return this.ensureBrowser();
    }
    this.starting = true;
    try {
      this.browser = await chromium.launch({
        headless: true,
        args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--disable-gpu', '--disable-extensions'],
      });
      return this.browser;
    } finally {
      this.starting = false;
    }
  }

  async createSession(sessionId: string, workspaceId: string): Promise<BrowserSession> {
    const browser = await this.ensureBrowser();
    const context = await browser.newContext({
      viewport: { width: 1280, height: 720 },
      userAgent: 'Brevio-AI-Agent/1.0 (+https://brevio.ai/bot)',
      javaScriptEnabled: true,
      acceptDownloads: false,
      permissions: [],
    });
    const session = new BrowserSession(context, sessionId, workspaceId);
    this.sessions.set(sessionId, session);
    return session;
  }

  getSession(sessionId: string): BrowserSession | undefined {
    return this.sessions.get(sessionId);
  }

  async closeSession(sessionId: string): Promise<void> {
    const session = this.sessions.get(sessionId);
    if (session) { await session.close(); this.sessions.delete(sessionId); }
  }

  async shutdown(): Promise<void> {
    for (const [id] of this.sessions) await this.closeSession(id);
    if (this.browser) { await this.browser.close(); this.browser = null; }
  }
}
