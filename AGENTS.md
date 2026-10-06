# Agents instructions

> If the design takes a long time to build, it's the wrong design. This is the fundamental thing. Over and over, the tendency is to complicate things.
> The best part is no part. The best code is no code. The best process is no process. Each weighs nothing, costs nothing, can't go wrong.
> Undesigning is the best thing. Just delete it. That's the best thing.

These are not explicit instructions, but a mantra to operate by.
Don't add needless complexity.
Do not create more code or process unless it resolves a concrete blocker.
Make it just as complex as is needed and move on.

## Code

### Testing

- Highly prefer e2e tests where testing is needed. Use them to verify complex features work.
- **NEVER** write unit tests after you write code.
- Tautological tests are considered harmful.
- Change-detector tests (unit tests that blindly mirror implementation details rather than verifying actual code behavior) are considered harmful.
- Do not create regression tests for bug fixes without a genuine gap in behavior testing.
- Before writing tests, first write down all the ways it could fail, then write the code.

#### E2E

source code -> binary -> docker image -> helm chart -> zarf/uds package

Scope e2e tests to the highest level possible (zarf/uds package in the above example).
If testing isn't possible at the highest level, or is impractical, ask yourself:

"Is this code really doing any work/providing any value if it's not visible to a user in any potential failure mode from their interface?"

If the answer is truly yes, then scope down as little as necessary that still verifies the code path works as intended.


Zarf/UDS package E2E tests are to be run on a k3d cluster using the default uds task (`uds run default`).
If a k3d cluster is already running, the `uds run dev` target will re-build and deploy the Zarf/UDS package.
If issues are encountered after running `uds run dev` that were not present in the initial run, first determine if the issue is because the cluster isn't fresh.
Spinning up a new k3d cluster is cheap relative to auditing code changes time/complexity wise, given the possibility that the issue is stale/borked cluster.

## References

[UDS Documentation](https://docs.defenseunicorns.com/llms.txt)
[Zarf Documentation](https://docs.zarf.dev/)

