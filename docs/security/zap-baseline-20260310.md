# OWASP ZAP API Scan — Baseline Report

**Date:** 2026-03-10
**Tool:** OWASP ZAP (ghcr.io/zaproxy/zaproxy:stable)
**Scan type:** API Scan with OpenAPI specification
**Target:** `http://localhost:2187/api/v1/`
**OpenAPI spec:** `docs/api/openapi.yaml`

## Summary

| Category | Count |
|----------|-------|
| Active scan rules tested | 53 |
| **PASS** | 52 |
| **WARN** | 1 |
| **FAIL** | 0 |

## Active Scan Results

### Injection Attacks

| Rule ID | Test | Result |
|---------|------|--------|
| 40018 | SQL Injection (Generic) | ✅ PASS |
| 40019 | SQL Injection — MySQL (Time Based) | ✅ PASS |
| 40020 | SQL Injection — Hypersonic SQL (Time Based) | ✅ PASS |
| 40021 | SQL Injection — Oracle (Time Based) | ✅ PASS |
| 40022 | SQL Injection — PostgreSQL (Time Based) | ✅ PASS |
| 40027 | SQL Injection — MsSQL (Time Based) | ✅ PASS |
| 90021 | XPath Injection | ✅ PASS |
| 90029 | SOAP XML Injection | ✅ PASS |
| 90017 | XSLT Injection | ✅ PASS |

### Cross-Site Scripting (XSS)

| Rule ID | Test | Result |
|---------|------|--------|
| 40012 | Cross Site Scripting (Reflected) | ✅ PASS |
| 40014 | Cross Site Scripting (Persistent) | ✅ PASS |
| 40016 | Cross Site Scripting (Persistent) — Prime | ✅ PASS |
| 40017 | Cross Site Scripting (Persistent) — Spider | ✅ PASS |
| 40026 | Cross Site Scripting (DOM Based) | ✅ PASS |

### Remote Code Execution

| Rule ID | Test | Result |
|---------|------|--------|
| 20018 | Remote Code Execution — CVE-2012-1823 | ✅ PASS |
| 40048 | Remote Code Execution (React2Shell) | ✅ PASS |
| 90019 | Server Side Code Injection | ✅ PASS |
| 90020 | Remote OS Command Injection | ✅ PASS |
| 90037 | Remote OS Command Injection (Time Based) | ✅ PASS |

### Server-Side Attacks

| Rule ID | Test | Result |
|---------|------|--------|
| 90023 | XML External Entity Attack | ✅ PASS |
| 40009 | Server Side Include | ✅ PASS |
| 90035 | Server Side Template Injection | ✅ PASS |
| 90036 | Server Side Template Injection (Blind) | ✅ PASS |
| 90026 | SOAP Action Spoofing | ✅ PASS |
| 40044 | Exponential Entity Expansion (Billion Laughs) | ✅ PASS |

### Path & File Attacks

| Rule ID | Test | Result |
|---------|------|--------|
| 6 | Path Traversal | ✅ PASS |
| 7 | Remote File Inclusion | ✅ PASS |
| 40032 | .htaccess Information Leak | ✅ PASS |
| 40034 | .env Information Leak | ✅ PASS |
| 40035 | Hidden File Finder | ✅ PASS |

### Authentication & Session

| Rule ID | Test | Result |
|---------|------|--------|
| 3 | Session ID in URL Rewrite | ✅ PASS |
| 20019 | External Redirect | ✅ PASS |
| 90033 | Loosely Scoped Cookie | ✅ PASS |

### Known CVEs

| Rule ID | Test | Result |
|---------|------|--------|
| 40043 | Log4Shell | ✅ PASS |
| 40045 | Spring4Shell | ✅ PASS |
| 90001 | Insecure JSF ViewState | ✅ PASS |
| 90002 | Java Serialization Object | ✅ PASS |

### Infrastructure

| Rule ID | Test | Result |
|---------|------|--------|
| 30001 | Buffer Overflow | ✅ PASS |
| 30002 | Format String Error | ✅ PASS |
| 40003 | CRLF Injection | ✅ PASS |
| 40008 | Parameter Tampering | ✅ PASS |
| 40028 | ELMAH Information Leak | ✅ PASS |
| 40029 | Trace.axd Information Leak | ✅ PASS |
| 40042 | Spring Actuator Information Leak | ✅ PASS |
| 90004 | Insufficient Site Isolation Against Spectre | ✅ PASS |
| 90011 | Charset Mismatch | ✅ PASS |
| 90022 | Application Error Disclosure | ✅ PASS |
| 90024 | Generic Padding Oracle | ✅ PASS |
| 90030 | WSDL File Detection | ✅ PASS |
| 90034 | Cloud Metadata Potentially Exposed | ✅ PASS |
| 90003 | Sub Resource Integrity Attribute Missing | ✅ PASS |
| 50000 | Script Active Scan Rules | ✅ PASS |
| 50001 | Script Passive Scan Rules | ✅ PASS |

### Warnings

| Rule ID | Test | Result | Details |
|---------|------|--------|---------|
| 100001 | Unexpected Content-Type | ⚠️ WARN | 13 instances — SPA fallback returns `text/html` for unknown paths. This is expected behavior: Vue Router handles client-side routing, so the server returns the SPA shell for any unrecognized path. Not a security issue. |

## How to Reproduce

```bash
# Start Capacitarr
make build

# Run ZAP API scan
make security:zap

# Reports generated:
#   zap-report.html  — full HTML report
#   zap-report.md    — markdown summary
```
