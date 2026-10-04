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

