/* eslint-disable no-console */
import fs from 'node:fs';
import path from 'node:path';

const seedPath = path.resolve(process.cwd(), 'config', 'skill-disambiguation.yaml');
if (!fs.existsSync(seedPath)) {
  console.error('missing skill-disambiguation.yaml');
  process.exit(1);
}

console.log('Skill seed scaffold verified:', seedPath);
console.log('TODO: implement 153-skill registry seed insertion.');
