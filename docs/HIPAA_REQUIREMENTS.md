# HIPAA Requirements Assessment

Health data is in scope. This document defines HIPAA Security Rule expectations and gaps.

## Role Determination
- If serving covered entities (providers/insurers), the product is a Business Associate.
- A Business Associate Agreement (BAA) is required before processing PHI.

## Required Safeguards
- Administrative: security policies, workforce training, risk analysis.
- Physical: access controls for infrastructure and data centers (handled by cloud providers).
- Technical:
  - Access control and unique user identification
  - Audit controls for PHI access
  - Integrity controls (tamper detection)
  - Transmission security (TLS)

## Data Handling
- Minimum necessary access for PHI.
- Encrypt PHI at rest and in transit.
- Log PHI access and modifications.

## Incident Response
- Breach notification process defined and tested.
- Retention of audit logs for at least 12 months.

## Vendor Management
- Ensure vendors support HIPAA (hosting, storage, communications).
- Maintain vendor list and DPAs/BAAs.

## Launch Gate
- HIPAA compliance review required before enabling health features for enterprise customers.
