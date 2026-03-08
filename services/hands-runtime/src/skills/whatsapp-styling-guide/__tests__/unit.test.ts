import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import adapter from '../index.js';

describe('whatsapp-styling-guide adapter', () => {
  it('requires text input', async () => {
    const result = await adapter.execute({}, {} as never);
    assert.equal(result.status, 'FAILED');
    assert.match(result.error?.message ?? '', /WHATSAPP_STYLING_TEXT_REQUIRED|Required/);
  });

  it('formats list output for bullet style', async () => {
    const result = await adapter.execute(
      { text: 'Milk; Eggs; Oat milk', style: 'bullet' },
      {} as never
    );

    assert.equal(result.status, 'SUCCESS');
    assert.equal(result.data?.provider, 'whatsapp-styling-guide');
    assert.match(result.data?.formatted_text ?? '', /• Milk/);
  });
});
