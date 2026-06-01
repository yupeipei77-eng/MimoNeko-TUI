# MioNeko Real World Benchmark

This document records a real MiMo API validation run for MioNeko. It is not a
mock benchmark and does not use simulated token data.

## Environment

| Field | Value |
|-------|-------|
| Provider | MiMo |
| Model | mimo-v2.5-pro |
| Base URL | https://token-plan-cn.xiaomimimo.com/v1 |
| Network | Real network environment |
| API Key | Real API Key, stored only in user-level config |

## Test

Command:

```powershell
mimoneko "Reply OK"
```

Observed output:

```text
Result:
OK

Tokens:
Input    70
Cached   64
Hit Rate 91.43%
```

Result:

| Metric | Value |
|--------|-------|
| InputTokens | 70 |
| CachedTokens | 64 |
| HitRate | 91.43% |
| Latency | 1473 ms |

Latency note: the `1473 ms` latency was reported by `mimoneko model test` in
the same real MiMo API environment immediately before the `mimoneko "Reply OK"`
run. The current `run` output reports token/cache metrics but does not expose a
separate per-run latency field.

## Cache Hit Rate Calculation

Cache hit rate is calculated from provider-reported token usage:

```text
HitRate = CachedTokens / InputTokens * 100
```

For this run:

```text
64 / 70 * 100 = 91.428571...
```

Rounded to two decimal places:

```text
91.43%
```

## Test Steps

1. Install or build MioNeko.
2. Configure MiMo authentication with a real API Key:

   ```powershell
   mimoneko auth login
   ```

3. Use the default MiMo connection:

   ```text
   Base URL: https://token-plan-cn.xiaomimimo.com/v1
   Model: mimo-v2.5-pro
   ```

4. Verify model connectivity:

   ```powershell
   mimoneko model test
   ```

5. Run the benchmark command:

   ```powershell
   mimoneko "Reply OK"
   ```

6. Record `Input`, `Cached`, and `Hit Rate` from the `Tokens` section.

## Reproduction

Use a real MiMo API Key and a real network connection. Do not use a mock server,
fake provider, or simulated benchmark output.

```powershell
mimoneko auth status
mimoneko config show
mimoneko model test
mimoneko "Reply OK"
```

Expected validation points:

- `auth status` shows `Status: Ready` or equivalent ready output.
- `config show` masks the API Key and stores it in the user-level config.
- `model test` succeeds against the real MiMo API.
- `mimoneko "Reply OK"` returns `OK`.
- Token metrics are provider-reported real values.

## Notes

- Data comes from the real MiMo API.
- Data is not from a mock.
- Data is not from a simulated benchmark.
- A real API Key was used, but the key is not recorded in this repository.
- Results can vary across time, account state, provider cache state, and network
  conditions.
