// Phase 3G.1 evidence — Production Hardening.
//
// 3G.1's PASS criterion per founder directive (2026-05-29) is:
//   "Each in-scope item needs one regression test that would have
//    caught the original incident shape AND one clear evidence/check
//    path proving the fix."
//
// Items #1 / #2 / #3 / #10 are all unit-tested. The four regression
// tests + the test count delta from main are the gate. This script
// performs the static checks that the runtime now supports the new
// shapes (new audit action, new memory_signal source, new pnpm
// migration script wired). It does NOT require any new audit rows to
// exist — those only fire when the fault occurs in the real run.
//
// Read-only. No DB writes. Safe to run any time.

import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { REQUIRED_TABLES, verifyMigrations } from '../src/db/migration-verifier.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_SOURCES } from '../src/memory/memory-signals.js';
import { loadDbClient } from '../src/db/client.js';

interface Finding {
  readonly severity: 'pass' | 'fail';
  readonly item: string;
  readonly detail: string;
}

async function main(): Promise<void> {
  console.log('Phase 3G.1 evidence — Production Hardening\n');
  const findings: Finding[] = [];

  /* ------------------------------------------------------------------ */
  /* Item #1 — migration verifier surface checks                        */
  /* ------------------------------------------------------------------ */

  if (REQUIRED_TABLES.length >= 13) {
    findings.push({
      severity: 'pass',
      item: '#1 migration verifier — REQUIRED_TABLES list complete',
      detail: `${REQUIRED_TABLES.length} required tables registered (≥13 expected)`
    });
  } else {
    findings.push({
      severity: 'fail',
      item: '#1 migration verifier — REQUIRED_TABLES incomplete',
      detail: `only ${REQUIRED_TABLES.length} required tables registered (≥13 expected)`
    });
  }

  // Migration-script wired in package.json
  const here = path.dirname(fileURLToPath(import.meta.url));
  const pkgPath = path.resolve(here, '..', 'package.json');
  try {
    const pkg = JSON.parse(await readFile(pkgPath, 'utf8')) as { scripts?: Record<string, string> };
    if (pkg.scripts && pkg.scripts['migrate:neon']) {
      findings.push({
        severity: 'pass',
        item: '#1 migration verifier — pnpm migrate:neon script wired',
        detail: pkg.scripts['migrate:neon']
      });
    } else {
      findings.push({
        severity: 'fail',
        item: '#1 migration verifier — pnpm migrate:neon script missing',
        detail: 'package.json scripts.migrate:neon not found'
      });
    }
  } catch (err) {
    findings.push({
      severity: 'fail',
      item: '#1 migration verifier — could not read package.json',
      detail: err instanceof Error ? err.message : String(err)
    });
  }

  /* ------------------------------------------------------------------ */
  /* Item #2 — OPTED_OUT decoder + new audit action                     */
  /* ------------------------------------------------------------------ */

  if ((FOMO_AUDIT_ACTIONS as readonly string[]).includes('fomo.send.opt_out_drift_detected')) {
    findings.push({
      severity: 'pass',
      item: '#2 SendBlue OPTED_OUT decoder — fomo.send.opt_out_drift_detected audit action registered',
      detail: 'allowlisted in core/audit.ts FOMO_AUDIT_ACTIONS'
    });
  } else {
    findings.push({
      severity: 'fail',
      item: '#2 SendBlue OPTED_OUT decoder — new audit action missing',
      detail: 'fomo.send.opt_out_drift_detected not present in FOMO_AUDIT_ACTIONS'
    });
  }

  if ((MEMORY_SIGNAL_SOURCES as readonly string[]).includes('opt_out_drift_carrier')) {
    findings.push({
      severity: 'pass',
      item: '#2 SendBlue OPTED_OUT decoder — opt_out_drift_carrier memory-signal source registered',
      detail: 'allowlisted in memory/memory-signals.ts MEMORY_SIGNAL_SOURCES'
    });
  } else {
    findings.push({
      severity: 'fail',
      item: '#2 SendBlue OPTED_OUT decoder — new memory-signal source missing',
      detail: 'opt_out_drift_carrier not present in MEMORY_SIGNAL_SOURCES'
    });
  }

  /* ------------------------------------------------------------------ */
  /* Item #3 — needs_reauth visibility (compile-time presence)          */
  /* ------------------------------------------------------------------ */

  // The presence of the helper file proves the boot WARN path exists;
  // its tests prove the cycle attr surface.
  try {
    await import('../src/workers/needs-reauth-boot-check.js');
    findings.push({
      severity: 'pass',
      item: '#3 needs_reauth visibility — findUsersNeedingReauth helper present',
      detail: 'src/workers/needs-reauth-boot-check.ts importable'
    });
  } catch (err) {
    findings.push({
      severity: 'fail',
      item: '#3 needs_reauth visibility — helper import failed',
      detail: err instanceof Error ? err.message : String(err)
    });
  }

  /* ------------------------------------------------------------------ */
  /* Item #10 — memory_signals snapshot (compile-time presence)         */
  /* ------------------------------------------------------------------ */

  try {
    await import('../src/workers/memory-signals-boot-snapshot.js');
    findings.push({
      severity: 'pass',
      item: '#10 memory_signals snapshot — snapshotMemorySignalsForBoot helper present',
      detail: 'src/workers/memory-signals-boot-snapshot.ts importable'
    });
  } catch (err) {
    findings.push({
      severity: 'fail',
      item: '#10 memory_signals snapshot — helper import failed',
      detail: err instanceof Error ? err.message : String(err)
    });
  }

  /* ------------------------------------------------------------------ */
  /* Optional: live Neon migration check (only when DATABASE_URL set)   */
  /* ------------------------------------------------------------------ */

  if ((process.env.DATABASE_URL ?? '').trim()) {
    const dbResult = loadDbClient({ env: process.env });
    if (dbResult.ok) {
      try {
        const result = await verifyMigrations(dbResult.client);
        if (result.ok) {
          findings.push({
            severity: 'pass',
            item: '#1 migration verifier — live Neon verification clean',
            detail: `all ${result.required_tables.length} required tables present`
          });
        } else {
          findings.push({
            severity: 'fail',
            item: '#1 migration verifier — live Neon has pending migrations',
            detail: result.missing_tables.map((m) => `${m.name} (${m.migration})`).join(', ')
          });
        }
      } catch (err) {
        findings.push({
          severity: 'fail',
          item: '#1 migration verifier — live check threw',
          detail: err instanceof Error ? err.message : String(err)
        });
      } finally {
        await dbResult.pool.end();
      }
    } else {
      findings.push({
        severity: 'fail',
        item: '#1 migration verifier — DATABASE_URL set but DB client failed',
        detail: dbResult.reason
      });
    }
  }

  /* ------------------------------------------------------------------ */
  /* Summary                                                            */
  /* ------------------------------------------------------------------ */

  console.log('========================================================================');
  console.log('Phase 3G.1 evidence summary');
  console.log('========================================================================');

  const failed = findings.filter((f) => f.severity === 'fail');
  for (const f of findings) {
    const mark = f.severity === 'pass' ? '✓' : '✖';
    console.log(`  [${mark}] ${f.item}`);
    console.log(`        ${f.detail}`);
  }
  console.log('');

  if (failed.length === 0) {
    console.log('VERDICT: PASS  (run `pnpm --filter @brevio/fomo run test` to confirm regression tests are green)');
    console.log('Phase 3G.1 — Production Hardening — static checks pass.');
    process.exit(0);
  }
  console.log(`VERDICT: FAIL  (${failed.length} static check(s) failed)`);
  process.exit(1);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
