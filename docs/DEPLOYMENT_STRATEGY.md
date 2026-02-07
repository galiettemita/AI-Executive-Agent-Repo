# Deployment Strategy

## Approach
- Rolling deploys for API services
- Background workers deployed separately

## Release Process
1. Merge to main
2. CI passes
3. Deploy to staging
4. Smoke tests
5. Promote to production

## Rollback
- Maintain previous image/version
- Automated rollback on health check failure
