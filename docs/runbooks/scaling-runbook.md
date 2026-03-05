# Scaling Runbook

## Purpose
Scale application and data layers safely under sustained load.

## Scale Signals
- HPA saturation for > 10 minutes.
- Queue backlog above thresholds.
- P95/P99 latency above SLO.
- Worker concurrency saturation.

## Procedure
1. Confirm demand pattern is real (not traffic anomaly).
2. Scale application tier.
   - Increase HPA max replicas for impacted services.
   - Verify pod scheduling capacity.
3. Scale cluster tier.
   - Increase node group bounds via Terraform.
   - Confirm Cluster Autoscaler convergence.
4. Scale stateful dependencies when required.
   - RDS class/storage updates.
   - Redis shard/replica scaling.
5. Re-run synthetic load and compare SLOs.
6. Record final settings and open capacity planning follow-up.

## Guardrails
- Never scale without monitoring rollback path.
- Keep one change domain at a time (app, then cluster, then data).
- Revert temporary overprovisioning once traffic normalizes.
