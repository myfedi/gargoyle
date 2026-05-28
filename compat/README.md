# Federation compatibility testing

This directory contains local interoperability fixtures for testing Gargoyle against other Fediverse implementations.

## GoToSocial local test setup

The included Compose file runs GoToSocial at `http://gts.test` and lets it reach a Gargoyle server exposed at `http://gargoyle.test` through a local reverse proxy.

### 1. Hostnames

Add local hostnames:

```sh
sudo sh -c 'echo "127.0.0.1 gargoyle.test gts.test" >> /etc/hosts'
```

### 2. Gargoyle config

Use a reverse-proxy-aware public host:

```yaml
domain: gargoyle.test
public_host: http://gargoyle.test
port: 3001
tls: false
```

`domain` is the account domain used for WebFinger (`acct:user@gargoyle.test`). `public_host` is the externally visible base URL used in actor IDs, inboxes, outboxes, and discovery links.

### 3. Reverse proxy

A ready-to-use Caddy config is included at [`Caddyfile.local`](Caddyfile.local):

```sh
sudo caddy run --config compat/Caddyfile.local
```

It proxies:

- `http://gargoyle.test` -> `127.0.0.1:3001`
- `http://gts.test` -> `127.0.0.1:8080`

Run Gargoyle separately:

```sh
go run cmd/web/server.go ./config.yml
```

### 4. Start GoToSocial

From the repo root:

```sh
docker compose -f compat/docker-compose.gts.yml up -d
```

The Compose file includes local-test-only settings:

```yaml
GTS_HTTP_CLIENT_ALLOW_IPS: "0.250.250.254/32"
GTS_HTTP_CLIENT_INSECURE_OUTGOING: "true"
```

These allow GoToSocial's federation HTTP client to fetch the host-gateway/reserved IP used by Docker in this setup. Do not use these settings for production.

### 5. Create a GTS account

```sh
docker exec -it gts /gotosocial/gotosocial admin account create \
  --username bob \
  --email bob@gts.test \
  --password 'Str0ngP@ssword!'

docker exec -it gts /gotosocial/gotosocial admin account promote \
  --username bob
```

### 6. Validated flows

Validated manually against GoToSocial:

| Flow | Result |
|---|---:|
| GoToSocial discovers Gargoyle via WebFinger | ✅ |
| GoToSocial fetches Gargoyle actor | ✅ |
| GoToSocial follows Gargoyle | ✅ |
| Gargoyle verifies signed inbound `Follow` | ✅ |
| Gargoyle fetches GoToSocial actor with signed GET | ✅ |
| Gargoyle sends signed `Accept` | ✅ |
| GoToSocial accepts follow | ✅ |
| Gargoyle sends public `Create/Note` | ✅ |
| GoToSocial receives/displays Gargoyle Note | ✅ |
| GoToSocial sends mention `Create/Note` | ✅ |
| Gargoyle verifies/stores inbound Note | ✅ |
| GoToSocial sends `Undo Follow` | ✅ |
| Gargoyle removes follower | ✅ |

### Notes from compatibility testing

The GTS run exposed compatibility requirements that are broadly useful across Fediverse implementations:

- Gargoyle needs separate bind settings and public URL settings when behind a reverse proxy.
- Some servers require signed actor fetches before returning actor documents.
- Outbound public Notes need audience fields (`to`/`cc`) on both the `Create` activity and `Note` object.
- For curl-based mention tests, use `--form-string` for statuses beginning with `@`.
