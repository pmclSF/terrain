# CLI design tokens

The canonical reference for how Terrain renders in a terminal. Every user-facing CLI surface — `terrain`, `terrain fix`, `terrain report`, the gate output — is built from the tokens below. They live in code at [`internal/uitokens`](../internal/uitokens/uitokens.go); this doc is the spec that package implements. Renderers consume the tokens; they never hand-roll ANSI escapes or glyphs. A future lint gate rejects raw escape sequences in user-visible paths.

The governing constraint: **you do not own the terminal.** Font, size, and background belong to the user's theme. Terrain's whole design vocabulary is foreground color, weight, a few glyphs, and whitespace on a monospace grid — and it must stay legible when color, Unicode, or width are unavailable.

## Color — seven semantic roles

Color carries **state**, never decoration. The roles are semantic, not literal: they name a job, and map to one of the 16 ANSI colors so they inherit the user's own theme (the way `git` and `gh` do). Never emit a 24-bit hex color — a hardcoded cyan that looks right on one theme is illegible on another.

| Role | Helper | ANSI | Carries |
|---|---|---|---|
| body | *(unwrapped)* | default fg | the sentence itself; inherits the theme foreground |
| dim | `Muted` | `90` bright-black | paths, labels, separators — present but never first |
| accent | `Accent` | `36` cyan | the value you'd copy: a field, a fix, an interpolated var |
| link | `Link` | `34` blue | a command you can run next (`terrain fix`) |
| success | `Ok` | `32` green | clean, in-sync, a line added, the `✓` |
| warning | `Warn` | `33` yellow | worth a look — drifting, deprecated, floating |
| danger | `Alert` | `31` red | breaks on merge, a line removed, coverage critically low |

`Bold` (`1`) is an **emphasis** modifier, not a role — it composes over any role to mark the one thing that matters in a block. Seven roles is the whole system; more reads as noise.

Semantic color (success / warning / danger) is separate from the accent. `accent` and `link` are the brand voice; the three state colors are meaning. Don't spend a state color on decoration.

## Glyphs — a small fixed set with an ASCII floor

Every glyph has an ASCII fallback. On a terminal that can't display UTF-8 (Windows conhost, a dumb terminal, a CI runner with no locale), the glyph degrades to plain ASCII so output is legible instead of mojibake. Route glyphs through the `Glyph*` accessors, never the raw `Sym*` constants.

| Meaning | Accessor | Unicode | ASCII floor |
|---|---|---|---|
| clean / resolved | `GlyphOK` | `✓` | `ok` |
| failed | `GlyphFail` | `✗` | `x` |
| warning | `GlyphWarn` | `⚠` | `!` |
| a finding | `GlyphFinding` | `●` | `*` |
| meter cell, filled | `GlyphMeterFull` | `●` | `#` |
| meter cell, empty | `GlyphMeterEmpty` | `○` | `-` |
| suggestion / next step | `GlyphChevron` | `›` | `>` |
| becomes / changes to | `GlyphArrow` | `→` | `->` |
| relates to (a contract) | `GlyphRelates` | `↔` | `<->` |
| item separator | `GlyphDot` | `·` | `-` |
| list bullet | `GlyphBullet` | `•` | `*` |
| prose dash | `GlyphDash` | `—` | `-` |
| metric improved | `GlyphUp` | `▲` | `^` |
| metric regressed | `GlyphDown` | `▼` | `v` |

A health meter is `GlyphMeterFull × n` + `GlyphMeterEmpty × (width−n)` — rendering `●●●○○○○○○○` on a capable terminal and `###-------` on the ASCII floor.

## Hierarchy without font sizes

You can't make anything bigger, so hierarchy comes from four levers, in order:

1. **Weight.** Bold the one thing that matters per block; dim everything supporting.
2. **Whitespace.** A blank line is a section break; indentation is the outline. Layout does the spacing.
3. **Caps labels.** `MAPPED`, `HEALTH` — quiet signposts (rendered `dim`), never shouted in color.
4. **Color, last.** Reach for it only to carry state, and only from the seven roles.

## Degradation — the plain path is the floor

The monochrome, ASCII, narrow rendering is what you design to; color and Unicode are the enhancement on top. Two independent capability flags gate it, each with its own trigger:

**Color** (`ColorEnabled`) is suppressed when any of:
- stdout is not a TTY (piped, redirected, or a CI log),
- `NO_COLOR` is set to any non-empty value ([no-color.org](https://no-color.org/)),
- `TERM=dumb`.

**Unicode** (`UnicodeEnabled`) is suppressed — falling back to the ASCII glyph floor — when either of:
- `TERRAIN_ASCII=1` is set,
- none of `LC_ALL` / `LC_CTYPE` / `LANG` advertises UTF-8.

These are orthogonal: a run can be full-color with ASCII glyphs (color TTY, non-UTF-8 locale) or monochrome with Unicode (piped from a UTF-8 shell). The design holds in all four corners because color only ever *reinforces* state that a glyph or a word already carries — it is never the sole channel. The biggest "terminal" Terrain runs in is CI, which is monochrome and often non-UTF-8; that path must be first-class, not a fallback afterthought.

## Width

Wide content — the map line, meters, tables — is sized against `TerminalWidth()`, which reads `COLUMNS` and clamps to `[40, 200]`, defaulting to `80` when unset or unparseable (the right assumption for a pipe or a CI log). Section rules use the fixed `SectionWidth` (60). Content that can still overflow a narrow terminal wraps or truncates (`Truncate`, `PadRight`) rather than blowing out the layout.

## The contract, in one line

Same bytes, every terminal: color when the terminal supports it, Unicode when the locale supports it, legible ASCII when neither does — and the meaning survives all the way down to monochrome ASCII at 80 columns, because color and glyphs reinforce state, they never solely carry it.
