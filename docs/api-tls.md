# TLS for the REST API

The REST API can serve HTTPS directly. It is **off by default** — terminating TLS
at a reverse proxy is a perfectly good arrangement, and it is what most
deployments already do.

The P2P layer is unaffected. It has always had its own mutually authenticated,
forward-secret encryption ([p2p-security.md](p2p-security.md)); this page is only
about the HTTP API.

---

## ⚙️ Enabling it

```ini
API_TLS_ENABLED=true
API_TLS_CERT_FILE=/path/to/cert.pem
API_TLS_KEY_FILE=/path/to/key.pem
```

The certificate and key are **loaded during configuration validation**, so a
missing or malformed pair fails at startup alongside every other configuration
problem — not when the listener comes up, and not on the first request.

---

## 🔐 The TLS profile

| Setting | Value | Why |
|---|---|---|
| Minimum version | **TLS 1.2** | Go permits 1.0 and 1.1 unless a minimum is set; both have been deprecated for years |
| Cipher suites | ECDHE + AEAD only | See below |

Only ECDHE AEAD suites are offered — AES-GCM and ChaCha20-Poly1305. Two families
are deliberately excluded:

- **CBC suites**, which have a long history of padding-oracle attacks.
- **RSA key exchange**, which provides no forward secrecy. Traffic recorded today
  becomes readable the day the server key leaks — the same property the P2P
  handshake avoids with ephemeral ECDH.

Go selects TLS 1.3 suites itself and ignores an explicit list for them, so only
the 1.2 suites are named.

`TestTLSProfileRefusesObsoleteVersions` checks every named suite is forward-secret
and AEAD, and cross-references Go's own list of insecure suites.

---

## 🧪 A certificate for local development

Either use `openssl`:

```bash
mkdir -p certs
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
  -keyout certs/key.pem -out certs/cert.pem -days 30 -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
```

or the SDK helper, which writes the same thing:

```go
certFile, keyFile, err := sdk.GenerateSelfSignedCert("./certs", []string{"localhost", "127.0.0.1"})
```

It produces a P-256 key in PKCS#8, valid for **30 days**, with the key mode `0600`
and the certificate `0644` — a certificate is public by definition, a key is not.
The short lifetime is deliberate: a self-signed certificate that quietly works for
a decade is one somebody eventually ships.

> **A self-signed certificate encrypts but authenticates nobody.** Any client
> that trusts it will equally trust an attacker presenting their own, so it stops
> passive eavesdropping and not an active machine-in-the-middle. Use a CA-issued
> certificate for anything another machine can reach.

Verify against it rather than disabling verification:

```bash
curl --cacert certs/cert.pem https://localhost:8100/v1/health
```

---

## 🔑 Encryption is not authorisation

TLS protects the connection. It says nothing about who is calling, and every
authenticated endpoint still requires its API key over HTTPS exactly as over
HTTP — `TestAuthenticationStillAppliesOverTLS` pins that.

---

## 🖥️ The CLI

`gbb-cli` reads `API_TLS_ENABLED` and picks `https://` accordingly. An explicit
`BLOCKCHAIN_API_URL` always wins, scheme included.

Previously it built an `http://` URL unconditionally, so enabling TLS left it
talking plaintext to a TLS listener — which fails as a bare `400` that says
nothing about the cause.

---

## 🔗 Related

- [API Reference](api.md)
- [P2P Security](p2p-security.md) — the peer transport, which is encrypted regardless
