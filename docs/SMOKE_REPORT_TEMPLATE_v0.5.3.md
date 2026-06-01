# Phase v0.5.3 Smoke Test Report — Production Hardening

> Filled after running every step in `smoke-test-v0.5.3-production-hardening.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.3.md` once `VERDICT: PASS` on all
> three evidence scripts (v0.5.1 + v0.5.2 + v0.5.3). **v0.5.3 PASS does
> NOT auto-unlock v1.0; the next phase runs its own 6Q gate.**

---

**Founder:** _<name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.3-production-hardening`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Alt-Gmail used (synthetic friend):** _<alt gmail you control — NOT a real friend>_
**Synthetic phone:** `+15550100099` (NANPA-reserved fictional)

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_v0.5.2.md` on `main` with `VERDICT: PASS`
- [ ] No real friend involved — alt Gmail of founder only
- [ ] No SendBlue plan upgrade — Free Sandbox stays
- [ ] Server has been running for at least one polling cycle on the v0.5.3 branch

## 2. PASS criteria (7)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| 1 | Each fix has a regression test tied to the original incident | 17 new tests across 4 files (gate, refresh-helper, pool, reconcile); all green in default + gated PG | ☐ |
| 2 | No manual SendBlue contact-add needed in the smoke | `/onboard/callback` audited `fomo.sendblue.contact_registered` or `_failed` automatically; `memory_signals.sendblue_contact_status` populated | ☐ |
| 3 | No manual OAuth refresh needed in the smoke | After forcing `expires_at` to the past, polling worker audited `fomo.oauth.refreshed` on the next cycle WITHOUT operator running `ops:refresh-oauth` | ☐ |
| 4 | Neon connection interruption does not crash the process | Pool error listener attached (verified via test); server uptime in the smoke window had no crash from any transient drop | ☐ |
| 5 | Missed SendBlue webhook can be detected by reconciliation | `pnpm ops:reconcile-sendblue` ran; result reported `gaps_found: N` and audited each gap as `fomo.sendblue.delivery_gap_detected` (N=0 if no gaps) | ☐ |
| 6 | v0.5.2 smoke path still passes | `smoke-evidence:v0.5.2` printed `VERDICT: PASS`; founder regression chain `detected → ranked → queued → approved → sent` still works | ☐ |
| 7 | No secrets / raw payloads / private data leaked | `smoke-evidence:v0.5.3` leak-canary scan over audit_log: zero hits for BREVIO_TOKEN_KEK material, connection strings | ☐ |

## 3. `smoke-evidence:v0.5.1` output

```
…
```

## 4. `smoke-evidence:v0.5.2` output

```
…
```

## 5. `smoke-evidence:v0.5.3` output

```
…
```

## 6. Operator-confirmed checks

| Check | Confirmed? | Notes |
|---|---|---|
| Polling worker auto-refreshed founder's access_token after forcing expiry | ☐ | `fomo.oauth.refreshed` audit with `provider: 'google'`, no refresh_token plaintext in detail |
| Friend's sendblue_contact_status memory_signal populated by `/onboard/callback` | ☐ | |
| Outbound worker refused to send when contact_status.registered=false | ☐ | `fomo.send.contact_not_registered` audit; alert transitioned `approved → failed` |
| Server stayed UP throughout the smoke window (no `process.exit(1)` from pool error) | ☐ | |
| `pnpm ops:reconcile-sendblue` produced sensible output (gap count + handles) | ☐ | |

## 7. Founder observations

| Observation | Note |
|---|---|
| Did the four fixes feel like the right shape? Anything you'd refactor before v0.5.4? | _<…>_ |
| Any bug surfaced during the smoke that's NOT one of the four items? | _<…>_ |
| If you ran a longer overnight test (e.g. 24h+), did the substrate stay alive? | _<…>_ |

## 8. Verdict

☐ **PASS** — all 7 criteria green, all three evidence scripts `VERDICT: PASS`, operator checks confirmed. **Next phase runs its own 6-question gate.**

☐ **FAIL** — list below.

Failures / followups:

- _…_

## 9. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_

## 10. What v0.5.3 PASS does NOT promise

- Periodic auto-reconciliation worker (still on-demand; future phase if desired)
- Production scaling beyond founder-on-laptop
- Second friend onboarding (its own gate)
- Auto-send, snooze, calendar, MCP, admin dashboard — all still out
- Android/SMS fallback — its own future smoke

The next phase is decided AT the next 6-question gate.
