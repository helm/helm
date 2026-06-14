# AGENTS.md — Task Rules

## Always Required

1. **Read the complete function before modifying it.** Use offset/limit or Grep to LOCATE
   the function, then read the entire body before writing any changes.

2. **No console.log / console.debug / console.info.** Remove them if already in existing code.

## Forbidden patterns — automatic review failure

**Timeout: zero-param async functions**
```js
// ❌ WRONG — { signal } is silently ignored if fn() takes no params
const result = await fn({ signal: controller.signal });

// ✅ CORRECT
let result;
try {
  result = await Promise.race([
    fn(),
    new Promise((_, r) => setTimeout(() => r(new Error('timeout')), 8000))
  ]);
} catch (_e) { result = fallback; }
```

**Error messages in responses**
```js
// ❌ WRONG — security violation, fails every review
res.send(`<p>${err.message || 'Unknown error'}</p>`);

// ✅ CORRECT — fixed string only
res.send('<p>Could not load data. Please refresh.</p>');
```

**Complete function bodies — no truncation**
```js
// ❌ WRONG
// ... rest of function unchanged

// ✅ Write every line from { to }
```

## Acceptance Criteria (must ALL be addressed)

- Add shellcheck CI workflow
- Fix shellcheck warnings in bash scripts
