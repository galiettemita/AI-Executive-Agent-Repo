/**
 * URL allowlist enforcement for browser automation.
 * Security: blocks internal IPs, enforces prefix matching, form_fill requires opt-in.
 */

export interface AllowlistEntry {
  origin: string;
  allowFormFill: boolean;
}

export class URLAllowlist {
  private readonly entries: AllowlistEntry[];

  constructor(entries: AllowlistEntry[]) {
    this.entries = Object.freeze([...entries]) as AllowlistEntry[];
  }

  static fromEnv(): URLAllowlist {
    const raw = process.env.BROWSER_ALLOWLIST_JSON ?? '[]';
    try {
      const parsed = JSON.parse(raw) as unknown[];
      const entries: AllowlistEntry[] = parsed.map((entry) => {
        if (typeof entry === 'string') {
          return { origin: entry, allowFormFill: false };
        }
        if (typeof entry === 'object' && entry !== null && 'origin' in entry) {
          const e = entry as Record<string, unknown>;
          return {
            origin: String(e.origin),
            allowFormFill: Boolean(e.allowFormFill ?? false),
          };
        }
        throw new Error(`invalid allowlist entry: ${JSON.stringify(entry)}`);
      });
      return new URLAllowlist(entries);
    } catch (err) {
      throw new Error(`BROWSER_ALLOWLIST_JSON parse error: ${err}`);
    }
  }

  validate(url: string, sessionType: string): string | null {
    const internalBlock = this.blockInternalIPs(url);
    if (internalBlock !== null) return internalBlock;

    let parsed: URL;
    try {
      parsed = new URL(url);
    } catch {
      return `INVALID_URL: ${url}`;
    }

    if (parsed.protocol !== 'https:') {
      const env = process.env.BREVIO_ENV ?? '';
      if (env !== 'local' && env !== 'test' && env !== '') {
        return `PROTOCOL_NOT_ALLOWED: only HTTPS permitted in ${env} environment`;
      }
    }

    const origin = `${parsed.protocol}//${parsed.host}`;
    const entry = this.entries.find((e) => origin === e.origin || url.startsWith(e.origin));

    if (!entry) {
      return `URL_NOT_IN_ALLOWLIST: ${origin} is not in the browser URL allowlist`;
    }

    if (sessionType === 'form_fill' && !entry.allowFormFill) {
      return `FORM_FILL_NOT_PERMITTED: ${origin} has not opted in to form_fill operations`;
    }

    return null;
  }

  private blockInternalIPs(url: string): string | null {
    const internalPatterns = [
      /^https?:\/\/localhost/i,
      /^https?:\/\/127\./,
      /^https?:\/\/10\./,
      /^https?:\/\/172\.(1[6-9]|2[0-9]|3[01])\./,
      /^https?:\/\/192\.168\./,
      /^https?:\/\/169\.254\./,
      /^https?:\/\/\[::1\]/,
      /^https?:\/\/\[fc/i,
      /^https?:\/\/0\.0\.0\.0/,
    ];
    for (const pattern of internalPatterns) {
      if (pattern.test(url)) {
        return `SSRF_BLOCKED: internal IP range blocked for ${url}`;
      }
    }
    return null;
  }

  getEntries(): Readonly<AllowlistEntry[]> {
    return this.entries;
  }
}
