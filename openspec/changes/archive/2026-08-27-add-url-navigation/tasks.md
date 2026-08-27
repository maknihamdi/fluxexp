## 1. URL encoding of the trail

- [x] 1.1 Add `encodeHop`/`decodeHop` in `app.js`: a hop is `type:ns:name`, or `domain:type:ns:name` when the domain is not `kubernetes`; each component is percent-encoded individually, with `/` and `|` restored afterwards for readability
- [x] 1.2 Add `encodeTrail`/`decodeTrail`: hops joined by `~`; an empty trail yields no `p` value; a hop with the wrong number of components makes decoding fail rather than yield a partial ref
- [x] 1.3 Add `labelFor(ref)` deriving the display label from the type (kind = part after the last `|`, then `kind ns/name`), mirroring `friendlyLabel` in `internal/ui/dto.go`, and use it wherever `ref.display` is currently assumed
- [x] 1.4 Build the query string by hand (`?context=…&p=…`) rather than through `URLSearchParams`, so `~` and `:` stay literal

## 2. The URL as the source of truth

- [x] 2.1 Replace `syncURL()` with `pushURL()` (history.pushState) and `replaceURL()` (replaceState, for the initial load only), both writing context + trail
- [x] 2.2 Rewrite `seedFromURL()` to read `context` and `p`, decode the trail into `state.trail`, and fall back to the roots home with a toast when decoding fails
- [x] 2.3 Register a `popstate` handler that re-reads the URL into `state` and re-renders, so Back/Forward walk the trail
- [x] 2.4 Make `drillTo`, the breadcrumb entries and the context `onchange` push a history entry instead of mutating state silently; context change clears the trail

## 3. Up-to-parent control

- [x] 3.1 Add the control to the breadcrumb bar in `index.html`/`app.js`, rendered only when the trail is non-empty
- [x] 3.2 Wire it to navigate to `trail.slice(0, -1)` with a push — a forward navigation, not `history.back()`
- [x] 3.3 Style it in `styles.css`, consistent with the existing breadcrumb links

## 4. New exploration

- [x] 4.1 Change `newExploration(ref)` to open the new `?context=…&p=<hop>` form and drop the `e_domain`/`e_type`/`e_ns`/`e_name`/`e_display` parameters
- [x] 4.2 Remove the legacy seed-parameter branch from the boot path

## 5. Verification

- [x] 5.1 `make vet` and `make test` stay green (no Go change expected — confirm the embedded assets still build)
- [x] 5.2 Manually verify on a smoke-test cluster: drill three levels, check the URL at each step; reload the deep URL and confirm the breadcrumb and node come back; Back/Forward walk the trail; the up control reaches the parent from a breadcrumb jump; context switch returns to the roots home
- [x] 5.3 Verify a deliberately corrupted `p` lands on the roots home with a toast, and that a hop pointing at a deleted object renders as an error node with the breadcrumb intact

## 6. Documentation

- [x] 6.1 Add the increment to the README roadmap and note the URL form
- [x] 6.2 Update the `internal/ui` section of `CLAUDE.md` with the URL-as-source-of-truth rule and the no-client-cache decision
