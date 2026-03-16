# OWASP ZAP API Scan — Baseline Report

**Date:** 2026-03-16
**Tool:** OWASP ZAP (ghcr.io/zaproxy/zaproxy:stable)
**Scan type:** API Scan with OpenAPI specification
**Target:** `http://localhost:2187/api/v1/`
**OpenAPI spec:** `docs/api/openapi.yaml`

## Summary

| Category | Count |
|----------|-------|
| Total scan rules tested | 119 |
| **PASS** | 119 |
| **WARN** | 1 |
| **FAIL** | 0 |

Of the 119 rules, 53 are active scan rules (attack simulation) and 66 are passive scan rules (observation-based analysis).

## Passive Scan Results

### Security Headers & Configuration

| Rule ID | Test | Result |
|---------|------|--------|
| 10010 | Cookie No HttpOnly Flag | ✅ PASS |
| 10011 | Cookie Without Secure Flag | ✅ PASS |
| 10015 | Re-examine Cache-control Directives | ✅ PASS |
| 10019 | Content-Type Header Missing | ✅ PASS |
| 10020 | Anti-clickjacking Header | ✅ PASS |
| 10021 | X-Content-Type-Options Header Missing | ✅ PASS |
| 10035 | Strict-Transport-Security Header | ✅ PASS |
| 10036 | HTTP Server Response Header | ✅ PASS |
| 10037 | Server Leaks Information via "X-Powered-By" | ✅ PASS |
| 10038 | Content Security Policy (CSP) Header Not Set | ✅ PASS |
| 10039 | X-Backend-Server Header Information Leak | ✅ PASS |
| 10054 | Cookie without SameSite Attribute | ✅ PASS |
| 10055 | CSP | ✅ PASS |
| 10056 | X-Debug-Token Information Leak | ✅ PASS |
| 10061 | X-AspNet-Version Response Header | ✅ PASS |
| 10063 | Permissions Policy Header Not Set | ✅ PASS |
| 10098 | Cross-Domain Misconfiguration | ✅ PASS |

### Information Disclosure

| Rule ID | Test | Result |
|---------|------|--------|
| 10009 | In Page Banner Information Leak | ✅ PASS |
| 10023 | Information Disclosure — Debug Error Messages | ✅ PASS |
| 10024 | Information Disclosure — Sensitive Information in URL | ✅ PASS |
| 10025 | Information Disclosure — Sensitive Information in HTTP Referrer Header | ✅ PASS |
| 10027 | Information Disclosure — Suspicious Comments | ✅ PASS |
| 10052 | X-ChromeLogger-Data (XCOLD) Header Information Leak | ✅ PASS |
| 10057 | Username Hash Found | ✅ PASS |
| 10062 | PII Disclosure | ✅ PASS |
| 10096 | Timestamp Disclosure | ✅ PASS |
| 10097 | Hash Disclosure | ✅ PASS |
| 10099 | Source Code Disclosure | ✅ PASS |
| 2 | Private IP Disclosure | ✅ PASS |

### Cross-Site & Redirect Attacks

| Rule ID | Test | Result |
|---------|------|--------|
| 10017 | Cross-Domain JavaScript Source File Inclusion | ✅ PASS |
| 10028 | Off-site Redirect | ✅ PASS |
| 10029 | Cookie Poisoning | ✅ PASS |
| 10030 | User Controllable Charset | ✅ PASS |
| 10031 | User Controllable HTML Element Attribute (Potential XSS) | ✅ PASS |
| 10043 | User Controllable JavaScript Event (XSS) | ✅ PASS |
| 10044 | Big Redirect Detected (Potential Sensitive Information Leak) | ✅ PASS |
| 10108 | Reverse Tabnabbing | ✅ PASS |

### Transport Security

| Rule ID | Test | Result |
|---------|------|--------|
| 10040 | Secure Pages Include Mixed Content | ✅ PASS |
| 10041 | HTTP to HTTPS Insecure Transition in Form Post | ✅ PASS |
| 10042 | HTTPS to HTTP Insecure Transition in Form Post | ✅ PASS |
| 10047 | HTTPS Content Available via HTTP | ✅ PASS |
| 10106 | HTTP Only Site | ✅ PASS |

### Authentication & Session

| Rule ID | Test | Result |
|---------|------|--------|
| 10105 | Weak Authentication Method | ✅ PASS |
| 10111 | Authentication Request Identified | ✅ PASS |
| 10112 | Session Management Response Identified | ✅ PASS |
| 10113 | Verification Request Identified | ✅ PASS |
| 10202 | Absence of Anti-CSRF Tokens | ✅ PASS |

### Known Vulnerabilities & Miscellaneous

| Rule ID | Test | Result |
|---------|------|--------|
| 0 | Directory Browsing | ✅ PASS |
| 10003 | Vulnerable JS Library (Powered by Retire.js) | ✅ PASS |
| 10026 | HTTP Parameter Override | ✅ PASS |
| 10032 | Viewstate | ✅ PASS |
| 10033 | Directory Browsing | ✅ PASS |
| 10034 | Heartbleed OpenSSL Vulnerability (Indicative) | ✅ PASS |
| 10045 | Source Code Disclosure — /WEB-INF Folder | ✅ PASS |
| 10048 | Remote Code Execution — Shell Shock | ✅ PASS |
| 10049 | Content Cacheability | ✅ PASS |
| 10050 | Retrieved from Cache | ✅ PASS |
| 10058 | GET for POST | ✅ PASS |
| 10104 | User Agent Fuzzer | ✅ PASS |
| 10109 | Modern Web Application | ✅ PASS |
| 10110 | Dangerous JS Functions | ✅ PASS |
| 10115 | Script Served From Malicious Domain (polyfill) | ✅ PASS |
| 10116 | ZAP is Out of Date | ✅ PASS |
| 20015 | Heartbleed OpenSSL Vulnerability | ✅ PASS |
| 20017 | Source Code Disclosure — CVE-2012-1823 | ✅ PASS |
| 100043 | Swagger UI Secret & Vulnerability Detector | ✅ PASS |

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

## Warnings

| Rule ID | Test | Result | Details |
|---------|------|--------|---------|
| 100001 | Unexpected Content-Type | ⚠️ WARN | 14 instances — SPA fallback returns `text/html` for unknown paths (including cloud metadata probe paths like `/computeMetadata/v1/`, `/latest/meta-data/`, `/metadata/instance`, `/metadata/v1`, `/opc/v1/instance/`, `/opc/v2/instance/`). This is expected behavior: Vue Router handles client-side routing, so the server returns the SPA shell for any unrecognized path. Not a security issue. |

## Informational Alerts (No Action Required)

| Alert | Risk Level | Instances | Notes |
|-------|------------|-----------|-------|
| Client Error response code (401) | Informational | 5 | Expected — unauthenticated API requests correctly return 401 Unauthorized |
| Non-Storable Content | Informational | 1 | 401 responses are correctly non-cacheable |

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
