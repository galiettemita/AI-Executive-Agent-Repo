import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('pdf-tools adapter', () => {
  it('requires two files for merge action', async () => {
    const result = await adapter.execute({ action: 'merge', files: ['one.pdf'] }, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /PDF_TOOLS_MERGE_FILES_REQUIRED/);
  });

  it('extracts text from a pdf file', async () => {
    const result = await adapter.execute({ action: 'extract_text', files: ['report.pdf'] }, {} as never);
    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'pdf-tools');
    assert.equal(typeof result.data?.extracted_text_preview, 'string');
  });
});
