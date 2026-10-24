# yarilo-autoconfig

Mail client autoconfiguration for yarilo. The service tells a mail client
which hosts and ports to use for an address:

- **Thunderbird** (desktop and Android), FairEmail: autoconfig XML, format 1.1;
- **Outlook** (classic, Windows): POX Autodiscover.

It is a separate service: it imports no yarilo code, calls no yarilo API and
keeps no list of domains. Every address gets the same provider hosts from the
configuration; addresses are never looked up, so the service cannot be used to
enumerate mailboxes.

## Endpoints

| Path | Method | Answer |
|:---|:---|:---|
| `/mail/config-v1.1.xml` | GET | Thunderbird autoconfig XML; `?emailaddress=` optional, a malformed one gets `400` |
| `/.well-known/autoconfig/mail/config-v1.1.xml` | GET | same |
| `/autodiscover/autodiscover.xml` (any case) | POST | Outlook POX settings; anything else, ActiveSync requests included, gets the POX error document (`600`) with `200` |
| `/autodiscover/autodiscover.xml` | GET | `405` |
| `/.well-known/caldav`, `/.well-known/carddav` | GET | `301` to `DAV_URL`, or `404` when it is empty |
| `/.well-known/jmap` | GET | `301` to `JMAP_URL`, or `404` when it is empty |

The `.well-known` redirects are a convenience for a user who types the
autoconfig host into a DAV or JMAP client. RFC 6764 and RFC 8620 discovery is
answered by the DAV and JMAP services on their own hosts.

Health and metrics are on the metrics port, which the ingress does not expose:
`/healthz`, `/readyz`, `/metrics`.

## Configuration

Environment variables. An empty variable keeps its default.

| Variable | Default | Meaning |
|:---|:---|:---|
| `IMAP_HOST` | --- | IMAP host, implicit TLS (required) |
| `IMAP_PORT` | `993` | |
| `SMTP_HOST` | --- | submission host (required) |
| `SMTP_IMPLICIT_TLS_PORT` | `465` | `0` = not offered |
| `SMTP_STARTTLS_PORT` | `587` | `0` = not offered; at least one SMTP port must be on |
| `POP3_HOST` | empty | empty = POP3 not offered |
| `POP3_PORT` | `995` | |
| `DISPLAY_NAME` | `Mail` | provider name shown by the client |
| `DAV_URL` | empty | `https` URL for the `.well-known/caldav` and `carddav` redirects |
| `JMAP_URL` | empty | `https` URL for the `.well-known/jmap` redirect |
| `LISTEN_ADDR` | `:8080` | client-facing listener |
| `METRICS_ADDR` | `:8081` | health and metrics listener |
| `LOG_LEVEL` | empty | `debug` logs each answered request with its domain (never the address) |

An invalid value stops the service at start with every problem listed.

Implicit TLS is offered before STARTTLS (RFC 8314 §4). Outlook takes one SMTP
entry: implicit TLS when it is on, otherwise STARTTLS. Thunderbird gets both,
implicit TLS first.

## Helm chart

`helm/` deploys a Deployment, a Service, an optional Ingress and a
PodDisruptionBudget. Every key under `config:` in `values.yaml` becomes the
environment variable of the same name upper-cased (`imap_host` → `IMAP_HOST`);
its numbers equal the defaults above, and a test keeps them equal.

| Value | Default | Meaning |
|:---|:---|:---|
| `image.repository` | `ghcr.io/yarilomail/yarilo-autoconfig` | |
| `image.tag` | empty | empty = the chart's `appVersion` |
| `imagePullSecrets` | `[]` | for a private registry mirror |
| `replicaCount` | `2` | |
| `config.*` | see above | `imap_host` and `smtp_host` are required: the chart does not render without them |
| `logLevel` | empty | `LOG_LEVEL` |
| `httpPort`, `metricsPort` | `8080`, `8081` | container ports |
| `service.port` | `80` | |
| `ingress.enabled` | `false` | |
| `ingress.className`, `ingress.annotations` | empty | |
| `ingress.hosts` | `autoconfig.example.net`, `autodiscover.example.net` | |
| `ingress.tlsSecretName` | empty | TLS for all hosts |
| `podDisruptionBudget.enabled`, `.minAvailable` | `true`, `1` | |
| `resources`, `nodeSelector`, `tolerations`, `affinity` | | as usual |

```sh
helm upgrade --install yarilo-autoconfig ./helm -n mail \
  --set config.imap_host=imap.example.net \
  --set config.smtp_host=smtp.example.net \
  --set ingress.enabled=true --set ingress.tlsSecretName=autoconfig-tls \
  --set 'imagePullSecrets[0].name=ghcr-pull'
```

## Host names and DNS

Thunderbird reaches the service through the domain's MX record: it takes the
first MX host, drops its first label and asks `autoconfig.<rest>`. With MX
`mx.example.net` the ingress host must be `autoconfig.example.net`. Renaming
`mx` to `mx01` changes nothing; moving the MX under another domain moves the
autoconfig host with it. A domain whose first MX is not the provider's (a
third-party spam filter) gets no autoconfig from this service.

Outlook reaches it through `_autodiscover._tcp.<customer domain>` SRV pointing
at `autodiscover.<provider>`. If `autodiscover.<customer domain>` resolves at
all, Outlook stops there with a certificate error and never reads the SRV
record.

## Metrics

| Metric | Labels |
|:---|:---|
| `autoconfig_requests_total` | `endpoint` (`autoconfig`, `autodiscover`, `well_known`, `other`), `status` |

Plus the Go and process collectors.

## Development

```sh
go test -race ./...
go test ./... -update   # rewrite testdata/*.golden after an intended change
helm lint helm -f hack/ci-values.yaml
```

## Releases

`appVersion` in `helm/Chart.yaml` is the release trigger, and `version` is kept
equal to it. A merge to `main` with a new value builds the image tagged
`<version>` (no `v`), creates the git tag `v<version>` and a GitHub release.
Every build on `main` also pushes `latest` and the short commit SHA.

## License

GNU AGPLv3, see [LICENSE](LICENSE).
