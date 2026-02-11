# Compliance Overview

## Targets
- SOC 2 Type II within 12-18 months (security, availability, confidentiality)
- GDPR readiness (EU launch later; build with GDPR controls now)
- HIPAA Security Rule compliance due to health data handling

## Scope Boundaries
- Jurisdiction: US-only launch (GDPR readiness but not active processing of EU residents yet)
- HIPAA: in scope because health data is handled; BAA required if acting as a Business Associate
- PCI: handled via Stripe, no card data stored locally
- COPPA: under-13 users are not permitted
- Enterprise: DPAs and security reviews required for enterprise customers

## Provider Policy Alignment (Must-Have)
- Meta/WhatsApp platform policies and message templates
- Google/Microsoft OAuth policies (limited use and minimum necessary scopes)
- OpenAI usage policies (safety + privacy requirements)
- Stripe policy compliance (payment flows, refunds, chargebacks)

## Operational Requirements
- Maintain vendor/subprocessor inventory (DPAs on file).
- Data retention enforcement aligned with DATA_RETENTION.md.
- User consent logging for all integrations.
- Right-to-access and right-to-delete workflows documented and tested.
