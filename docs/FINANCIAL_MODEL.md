# Financial Model Framework

## Assumptions
- Price: $29/month
- Year 1 users: 10,000
- Target cost per user: $5/month
- Heavy features: WhatsApp messaging, email/calendar sync, file/photo upload, vector search

## Revenue Projection (Base Case)
- Monthly recurring revenue (MRR) = Price * Active Users
- Example: $29 * 10,000 = $290,000 MRR
- Annual recurring revenue (ARR) = MRR * 12 = $3.48M

## Cost Breakdown (Targets)
- AI model usage: $2.00/user/month
- Messaging + integrations: $0.75/user/month
- Hosting/DB/storage: $1.25/user/month
- Observability/security/compliance: $0.50/user/month
- Total target cost: $4.50/user/month

## Sensitivity Analysis
- If costs rise to $8/user/month, gross margin drops.
- If conversion rate drops by 20%, ARR reduces proportionally.
- If WhatsApp costs spike, cap usage or limit message volume.

## Break-Even Analysis
- Break-even users = Fixed Costs / (Price - Variable Cost)
- Example (placeholder):
  - Fixed costs: $60,000/month
  - Variable cost: $5/user/month
  - Break-even users = 60,000 / (29 - 5) = 2,500 users

## Next Steps
- Validate pricing with customer interviews.
- Track actual model + infra costs per user.
- Update break-even monthly once real costs are known.
