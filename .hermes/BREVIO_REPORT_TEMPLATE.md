# Brevio Report Template

Every Brevio cycle report must use this compact evidence format.

- **Harness anchor loaded:** BREVIO-HARNESS-V1-NO-CIRCLING-FAST-SHIPPING
- **Current phase:**
- **Current NEXT queue item:**
- **What shipped:**
- **Branch:**
- **Commit SHA:**
- **PR number:**
- **Changed files:**
- **Validation tied to exact commit/PR:**
- **CI result:**
- **Founder gates touched?** yes/no
- **Did Hermes circle?** yes/no
- **If yes, what harness fix prevents repeat?**
- **Repeated blocker?** yes/no
- **Next concrete PR:**
- **Founder approval needed?** yes/no

## Loop detector

Before reporting, answer internally and correct course if any answer shows circling:

1. Did I ask for approval already granted?
2. Did I reopen a settled decision?
3. Did I spend the cycle on docs/meta-work without a shipping reason?
4. Did I rediscover a known blocker?
5. Did I cite unrelated CI for the current commit?
6. Did I end without a PR, changed files, or real blocker?
7. Did I make the next action more specific than “continue M1”?

If the same blocker appears twice, add a durable harness fix or one exact owner/action. Do not keep reporting the same blocker.

## Validation rule

Every validation statement must attach to the exact commit or PR it validates.

Correct: `PR #75 commit de54b57f: CI build + test passed.`

Incorrect: `PR #74 passed, so this local commit is okay.`
