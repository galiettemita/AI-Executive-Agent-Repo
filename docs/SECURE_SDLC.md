# Secure SDLC Controls

## Code Review
- All changes require PR review before merge.
- Require CI checks to pass before merge.

## Branch Protection
- Protect `main` and require 1+ approvals.
- Require signed commits if possible.

## CI Security
- CodeQL and dependency scanning enabled.
- Trivy filesystem + config scans enabled.
- OWASP ZAP baseline scan enabled (requires ZAP_TARGET_URL secret).
- Review security alerts weekly.
