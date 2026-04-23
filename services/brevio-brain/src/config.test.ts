import assert from 'node:assert/strict';
import path from 'node:path';
import { describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { loadDisambiguationRules } from './config.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const configPath = path.join(repoRoot, 'config', 'skill-disambiguation.yaml');

describe('brain config', () => {
  it('loads the full disambiguation ruleset including nested preference maps', () => {
    const rules = loadDisambiguationRules(configPath);
    assert.equal(Object.keys(rules).length, 20);
    assert.equal(rules['email-send']?.by_preference?.google, 'google-workspace');
    assert.equal(rules['spotify']?.terminal, 'spotify-player');
    assert.equal(rules['speech-transcription']?.canonical, 'asr');
    assert.equal(rules['document-perception']?.extract, 'pdf-tools');
  });
});
