# Design

## Visual Direction

A restrained product UI with a personal publishing feel: light, warm-neutral surfaces; compact but breathable layouts; familiar controls; and small moments of character in empty states and status language. Avoid decorative dashboard tropes.

## Theme

Default to a light theme for people checking and posting from normal desktop or laptop environments during everyday use. Support dark mode through tokens later, but do not make dark mode the primary identity.

## Color

Use OKLCH CSS variables. Keep the palette restrained: warm tinted neutrals, one grounded accent for primary actions and selection, and semantic colors for status. Avoid pure black and pure white.

## Typography

Use a system UI font stack for reliability and native feel. Keep hierarchy clear but modest, with labels and data optimized for scanning.

## Layout

Use a standard app shell with a left sidebar on desktop and a compact mobile navigation pattern. Pages should combine high-level summaries with task-focused tables, lists, and forms. Avoid nested cards and repetitive identical card grids.

## Components

Build with shadcn/ui and Radix primitives. Components must include accessible focus states, disabled states, and loading/empty/error variants where relevant.

## Motion

Use short state transitions only, around 150-200ms, respecting reduced-motion preferences. Motion should clarify feedback, reveal, or loading state, never decorate.
