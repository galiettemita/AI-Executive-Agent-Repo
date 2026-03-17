import { URLAllowlist } from '../allowlist';

describe('URLAllowlist', () => {
  const entries = [
    { origin: 'https://example.com', allowFormFill: true },
    { origin: 'https://read-only.com', allowFormFill: false },
  ];

  test('allows listed URL', () => {
    const al = new URLAllowlist(entries);
    expect(al.validate('https://example.com/page', 'scrape')).toBeNull();
  });

  test('blocks unlisted URL', () => {
    const al = new URLAllowlist(entries);
    const result = al.validate('https://evil.com', 'scrape');
    expect(result).toContain('URL_NOT_IN_ALLOWLIST');
  });

  test('allows form_fill on opted-in domain', () => {
    const al = new URLAllowlist(entries);
    expect(al.validate('https://example.com/form', 'form_fill')).toBeNull();
  });

  test('blocks form_fill on non-opted-in domain', () => {
    const al = new URLAllowlist(entries);
    const result = al.validate('https://read-only.com/form', 'form_fill');
    expect(result).toContain('FORM_FILL_NOT_PERMITTED');
  });

  test('blocks localhost unconditionally', () => {
    const al = new URLAllowlist([{ origin: 'http://localhost:3000', allowFormFill: true }]);
    const result = al.validate('http://localhost:3000/test', 'scrape');
    expect(result).toContain('SSRF_BLOCKED');
  });

  test('blocks internal IP 10.x', () => {
    const al = new URLAllowlist([{ origin: 'http://10.0.0.1', allowFormFill: false }]);
    expect(al.validate('http://10.0.0.1/api', 'scrape')).toContain('SSRF_BLOCKED');
  });

  test('blocks 192.168.x range', () => {
    const al = new URLAllowlist([]);
    expect(al.validate('http://192.168.1.1/', 'scrape')).toContain('SSRF_BLOCKED');
  });

  test('fromEnv with valid JSON', () => {
    process.env.BROWSER_ALLOWLIST_JSON = JSON.stringify([
      { origin: 'https://test.com', allowFormFill: false },
    ]);
    const al = URLAllowlist.fromEnv();
    expect(al.validate('https://test.com/page', 'scrape')).toBeNull();
    delete process.env.BROWSER_ALLOWLIST_JSON;
  });

  test('fromEnv with plain string entries', () => {
    process.env.BROWSER_ALLOWLIST_JSON = JSON.stringify(['https://simple.com']);
    const al = URLAllowlist.fromEnv();
    expect(al.validate('https://simple.com/page', 'scrape')).toBeNull();
    expect(al.validate('https://simple.com/form', 'form_fill')).toContain('FORM_FILL_NOT_PERMITTED');
    delete process.env.BROWSER_ALLOWLIST_JSON;
  });

  test('empty allowlist blocks everything', () => {
    const al = new URLAllowlist([]);
    expect(al.validate('https://example.com', 'scrape')).toContain('URL_NOT_IN_ALLOWLIST');
  });
});
