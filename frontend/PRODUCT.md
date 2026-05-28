# Product

## Register

product

## Users

Gargoyle serves people operating or using a small ActivityPub instance: instance admins, local posting users, and federation/debug operators. They may be managing configuration, checking federation behavior, reviewing inbox/outbox activity, posting notes, or resolving delivery issues.

## Product Purpose

Gargoyle is a nondogmatic, hackable, pragmatic ActivityPub server for single-user or small-group federation. The frontend should make day-to-day instance operation and personal publishing clear without exposing unnecessary protocol complexity. Success means users can post, understand federation state, manage follows, and diagnose issues with confidence.

## Brand Personality

Personal, calm, practical. The interface should feel approachable and human, with enough operational detail for trust, but not like a niche protocol debugger unless the user enters debugging views.

## Anti-references

Do not make this look like generic dark SaaS, a dense enterprise admin panel, a neon hacker terminal, or a developer-only protocol console. Avoid jargon-first navigation, decorative complexity, and anything that makes personal publishing feel like infrastructure work.

## Design Principles

1. Personal first, protocol second: foreground people, posts, follows, and actions; reveal ActivityPub details where they help.
2. Operational confidence without admin fatigue: show health, queues, and federation state clearly, without overwhelming users.
3. Small-instance warmth: optimize for one person or a small community, not enterprise scale.
4. Hackable structure: keep code modular, explicit, and easy to refactor into another React framework later.
5. Secure by default: treat all remote content as untrusted, avoid unsafe rendering, and make sensitive actions deliberate.

## Accessibility & Inclusion

Target WCAG 2.1 AA. Preserve keyboard navigation, visible focus states, semantic HTML, sufficient contrast, reduced-motion support, and layouts that work across desktop and mobile widths.
