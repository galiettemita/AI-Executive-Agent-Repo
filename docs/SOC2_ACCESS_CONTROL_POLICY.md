# SOC 2 Access Control Policy

## Purpose
Ensure only authorized users and services can access systems and data.

## Account Lifecycle
- Provision access based on role and business need.
- Remove access within 24 hours of separation or role change.
- Use unique accounts; shared accounts are not permitted.

## Authentication
- MFA required for admin and infrastructure accounts.
- Strong passwords enforced by the identity provider.
- Tokens and API keys must have explicit expiration where supported.

## Authorization
- Least-privilege access is mandatory.
- Admin access requires approval and is time-bound when possible.
- Service accounts are scoped to a single function.

## Reviews
- Quarterly access reviews for all admin and service accounts.
- Document findings and remediation actions.

## Logging
- Log all admin access and changes to permissions.
