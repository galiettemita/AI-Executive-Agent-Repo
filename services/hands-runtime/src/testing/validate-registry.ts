#!/usr/bin/env npx tsx
/**
 * validate-registry.ts — Validates all registered skills have correct structure.
 * Checks: non-empty id, valid inputSchema, valid outputSchema, execute function,
 * requiredScopes array, healthCheck function.
 *
 * Usage: npx tsx src/testing/validate-registry.ts
 * Exit code 0 = all pass, 1 = failures detected.
 */

import * as fs from 'node:fs';
import * as path from 'node:path';

interface ValidationError {
  skillId: string;
  issue: string;
}

async function main() {
  const skillsDir = path.resolve(import.meta.dirname ?? '.', '..', 'skills');
  if (!fs.existsSync(skillsDir)) {
    console.error(`Skills directory not found: ${skillsDir}`);
    process.exit(1);
  }

  const entries = fs.readdirSync(skillsDir, { withFileTypes: true });
  const errors: ValidationError[] = [];
  let checked = 0;
  let skipped = 0;

  for (const entry of entries) {
    if (!entry.isDirectory()) continue;
    if (entry.name.startsWith('_') || entry.name.startsWith('.')) {
      skipped++;
      continue;
    }

    const indexPath = path.join(skillsDir, entry.name, 'index.ts');
    if (!fs.existsSync(indexPath)) {
      // No index.ts = not a skill adapter directory
      skipped++;
      continue;
    }

    const skillId = entry.name;
    checked++;

    try {
      const mod = await import(path.join(skillsDir, entry.name, 'index.js'));
      const adapter = mod.default;

      if (!adapter) {
        errors.push({ skillId, issue: 'no default export' });
        continue;
      }

      // Check id
      if (!adapter.id || typeof adapter.id !== 'string') {
        errors.push({ skillId, issue: 'missing or non-string id' });
      }

      // Check inputSchema
      if (!adapter.inputSchema || typeof adapter.inputSchema !== 'object') {
        errors.push({ skillId, issue: 'missing or invalid inputSchema' });
      } else if (!adapter.inputSchema.properties && !adapter.inputSchema.type) {
        errors.push({ skillId, issue: 'inputSchema has no properties or type' });
      }

      // Check outputSchema
      if (!adapter.outputSchema || typeof adapter.outputSchema !== 'object') {
        errors.push({ skillId, issue: 'missing or invalid outputSchema' });
      } else if (!adapter.outputSchema.required || !Array.isArray(adapter.outputSchema.required)) {
        errors.push({ skillId, issue: 'outputSchema missing required array' });
      }

      // Check execute
      if (typeof adapter.execute !== 'function') {
        errors.push({ skillId, issue: 'execute is not a function' });
      }

      // Check requiredScopes
      if (!Array.isArray(adapter.requiredScopes)) {
        errors.push({ skillId, issue: 'requiredScopes is not an array' });
      }

      // Check healthCheck
      if (typeof adapter.healthCheck !== 'function') {
        errors.push({ skillId, issue: 'healthCheck is not a function' });
      }
    } catch (e: any) {
      // Import failures are common for skills that need build step
      // Only flag if the .ts file exists but can't be loaded
      errors.push({ skillId, issue: `import failed: ${e.message?.slice(0, 100)}` });
    }
  }

  console.log(`\nRegistry Validation Results:`);
  console.log(`  Checked: ${checked}`);
  console.log(`  Skipped: ${skipped}`);
  console.log(`  Errors:  ${errors.length}`);

  if (errors.length > 0) {
    console.log('\nFailures:');
    for (const err of errors) {
      console.log(`  ✗ ${err.skillId}: ${err.issue}`);
    }
    process.exit(1);
  }

  console.log('\n✓ All skills validated successfully.');
  process.exit(0);
}

main().catch(e => {
  console.error('Registry validation crashed:', e);
  process.exit(1);
});
