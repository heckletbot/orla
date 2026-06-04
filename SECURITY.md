# Security Policy

We take security vulnerabilities seriously. If you discover a security vulnerability, please follow these steps:

1. Do NOT open a public GitHub issue
2. Do NOT discuss the vulnerability publicly until it has been addressed
3. Email security concerns to hayder@alumni.harvard.edu
   - Include a detailed description of the vulnerability
   - Include steps to reproduce (if applicable)
   - Include potential impact assessment
   - Include suggested fix (if available)

### response timeline

- Initial Response within 2 business days
- Status Update within 7 business days
- The resolution depends on severity and complexity

### disclosure policy

- We will acknowledge receipt of your report within 48 hours
- We will provide regular updates on the status of the vulnerability
- Once the vulnerability is fixed, we will:
  - Credit you (if desired) in the security advisory
  - Publish a security advisory with details

## security best practices

When deploying orla:

- Keep orla updated to the latest version
- Run orla behind an authenticating reverse proxy (nginx, Cloudflare, an auth gateway) — orla itself does not enforce authentication on its API
- Use Postgres credentials with the minimum privileges orla requires (read/write on its own database; no superuser)
- Protect the upstream LLM/tool API keys orla holds in environment variables — restrict who can read the process environment and avoid committing `.env` files
- Monitor `/metrics` and structured logs for anomalous request patterns

## known security considerations

1. **No built-in auth.** The HTTP API trusts every caller. A reverse proxy or service mesh must enforce identity.
2. **API key fan-out.** orla holds outbound credentials for every backend; compromise of the orla process exposes all of them.
3. **Rate limiting is per-instance.** `rate_per_second` is enforced per orla process — multiple replicas multiply the effective cap.
4. **Tenant isolation is advisory.** Tenancy is carried by request headers (`X-Orla-Tag-Tenant`) and used for fair-share scheduling; it is not a security boundary on its own.

Thank you for helping keep orla secure!
