# Gargoyle Frontend Plan

## Goal

Build a Bun-powered Vite React frontend for Gargoyle using TypeScript, Tailwind CSS, and shadcn/ui. Keep the app modular so it can be refactored into Next.js later without rewriting feature code.

## Product Direction

- Product/admin UI, not marketing.
- Serve instance admins, local posting users, and federation/debug operators.
- Feel personal, calm, and practical, not too nerdy.
- Prioritize security, accessibility, clean code, and framework portability.

## Architecture

- Use Vite + React + TypeScript.
- Use Bun for dependency management and scripts.
- Organize code by reusable layers:
  - `src/app`: app shell, providers, router setup.
  - `src/components`: reusable UI and shadcn components.
  - `src/features`: product areas such as overview, posts, follows, inbox, outbox, delivery, settings.
  - `src/lib`: utilities, API client, config.
  - `src/types`: shared domain and API types.
- Keep business logic out of route files to ease a future Next.js migration.

## Initial Implementation Steps

1. Scaffold Vite React TypeScript with Bun.
2. Add Tailwind CSS and shadcn/ui configuration.
3. Add base UI components needed for the shell.
4. Create a first app shell with sidebar navigation and responsive layout.
5. Create initial placeholder feature screens:
   - Overview
   - Posts
   - Follows
   - Inbox
   - Outbox
   - Delivery
   - Compatibility
   - Settings
6. Add API client scaffolding with safe defaults and typed errors.
7. Gate the entire product UI behind Gargoyle OAuth.
8. Prepare for Mastodon-compatible APIs using the OAuth access token.
9. Add lint/build checks and fix issues.

## Security Notes

- Treat remote ActivityPub content as untrusted.
- Do not render remote HTML with `dangerouslySetInnerHTML`.
- Keep API base URL explicit and environment-driven.
- Avoid leaking secrets into client environment variables.
- Use OAuth Authorization Code with PKCE for browser auth, not client secrets.
- Validate OAuth `state` before completing authorization.
- Do not render authenticated product surfaces until an access token exists.
- Use typed request helpers and defensive error handling.

## Accessibility Notes

- Use semantic landmarks and headings.
- Preserve visible focus states.
- Ensure keyboard-accessible navigation.
- Maintain sufficient contrast using OKLCH tokens.
- Respect reduced-motion preferences.

## Next Decisions

- Confirm supported Mastodon-compatible endpoints and any Gargoyle-specific extensions.
- Confirm OAuth authorization, token refresh, revocation, and scopes.
- Decide whether Go serves the static frontend bundle or a separate host does.
