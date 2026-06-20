# Spec: Declarative Picker Navigation (Computed Flow Slice)

## Status

`draft`

## Capability Deltas

**None.** This is a pure internal refactor. No new or modified product capabilities. All requirements below are behavior-preservation invariants.

---

## Invariants

### INV-1 — Forward navigation visits only eligible screens in order

The conditional picker chain must be visited in this fixed order, skipping any screen whose `shouldShow*` predicate returns false for the current `m.Selection`:

```
Claude → Kiro → Codex → SDDMode → ModelPicker → StrictTDD → OpenCodePlugins → DependencyTree
```

`ModelPicker` is included only when `SDDMode == Multi` AND the model cache file is present (`osStatModelCache` returns no error).

No other screen may appear between these, and none may be skipped unless its predicate is false.

### INV-2 — Backward navigation is the exact reverse

Pressing Esc OR selecting the "← Back" row on any picker screen must navigate to the immediately preceding screen in the filtered slice — the same slice used for forward navigation. Both mechanisms must produce identical results.

Specific regression cases that must hold:

- **INV-2a:** Pressing Esc or "← Back" on `ScreenSDDMode` must return to `ScreenCodex` when Codex is present in the flow (i.e., `shouldShowCodex` is true). It must NOT skip Codex.
- **INV-2b:** The "← Back" row on `ScreenCodex` must navigate backward (to `ScreenKiro` or `ScreenClaude` per the slice). It must NOT be inert.

### INV-3 — PresetCustom ordering inversion

When `Preset == PresetCustom`, `ScreenDependencyTree` precedes the picker sequence (component selector first). The slice must reflect this ordering inversion; DependencyTree is NOT appended at the end in custom mode.

### INV-4 — ModelConfigMode exit-ramp

When pickers are entered from `ScreenModelConfig`, pressing Esc or "← Back" on the first picker in the flow must return to `ScreenModelConfig`, not to the previous picker-flow screen. This exit-ramp is a guard outside the slice and must remain unaffected.

### INV-5 — OpenCodePluginsStandalone guard

The `OpenCodePluginsStandalone` early-return guard must be preserved. It is not folded into the picker flow slice.

### INV-6 — ModelPicker condition uses injectable stat

`ScreenModelPicker` inclusion in the slice must call the package-level `osStatModelCache` variable (injectable in tests), not a hardcoded `os.Stat` call. The injectable boundary must not be removed.

### INV-7 — Full round-trip symmetry for any agent subset

For any combination of selected agents, navigating forward through the entire slice and then backward through the entire slice must return to the starting screen. The set of screens visited forward equals the set visited backward (in reverse order).

---

## Acceptance Scenarios

All scenarios are enforced by the existing test suite. No new external behavior is observable.

### Scenario 1 — All agents selected, full forward pass

**Given** all `shouldShow*` predicates return true and `SDDMode == Multi` with model cache present  
**When** the user confirms each picker screen in sequence  
**Then** the screens visited in order are: Claude → Kiro → Codex → SDDMode → ModelPicker → StrictTDD → OpenCodePlugins → DependencyTree

### Scenario 2 — Kiro excluded

**Given** `shouldShowKiro` is false, all others true, `SDDMode == Multi`, model cache present  
**When** the user confirms Claude and continues  
**Then** the next screen is Codex (Kiro is skipped); subsequent order is Codex → SDDMode → ModelPicker → StrictTDD → OpenCodePlugins → DependencyTree

### Scenario 3 — SDDMode back returns to Codex (regression INV-2a)

**Given** Codex is in the flow (`shouldShowCodex` true), the user has reached ScreenSDDMode  
**When** the user presses Esc OR selects "← Back" on ScreenSDDMode  
**Then** the current screen is ScreenCodex

### Scenario 4 — Codex "← Back" row is not inert (regression INV-2b)

**Given** Kiro is in the flow (`shouldShowKiro` true), the user is on ScreenCodex  
**When** the user selects "← Back" on ScreenCodex  
**Then** the current screen is ScreenKiro (not ScreenCodex)

### Scenario 5 — ModelPicker excluded when SDDMode != Multi

**Given** `SDDMode != Multi` (or model cache absent)  
**When** the user confirms SDDMode  
**Then** the next screen is StrictTDD (ModelPicker is not visited)

### Scenario 6 — ModelPicker excluded when model cache absent

**Given** `SDDMode == Multi` but `osStatModelCache` returns an error  
**When** the user confirms SDDMode  
**Then** the next screen is StrictTDD (ModelPicker is not visited)

### Scenario 7 — PresetCustom ordering inversion

**Given** `Preset == PresetCustom`  
**When** navigation enters the picker chain  
**Then** DependencyTree appears first (before Claude/Kiro/Codex), not last

### Scenario 8 — ModelConfigMode exit-ramp

**Given** pickers were entered from ScreenModelConfig  
**When** the user presses Esc on the first picker screen  
**Then** the current screen is ScreenModelConfig

### Scenario 9 — Full round-trip symmetry (all agents)

**Given** all agents selected, `SDDMode == Multi`, model cache present  
**When** the user navigates forward through all picker screens, then backward through all picker screens  
**Then** the sequence of screens visited backward is the exact reverse of the forward sequence, and the user returns to the pre-picker screen

### Scenario 10 — Full round-trip symmetry (partial subset)

**Given** only Claude and StrictTDD are selected (all other predicates false, `SDDMode != Multi`)  
**When** the user navigates forward then backward  
**Then** only Claude and StrictTDD appear in both passes; no other screen is visited

---

## Test Enforcement

| Scenario(s) | Enforced by |
|-------------|-------------|
| 1, 2, 9, 10 | `internal/tui/preset_flow_test.go::TestInstallNavigationRoundTrips` (all 11 cases) |
| 3 (INV-2a) | `TestInstallNavigationRoundTrips` case: SDDMode back regression |
| 4 (INV-2b) | `TestInstallNavigationRoundTrips` case: Codex back regression |
| 5, 6 (INV-6) | `pickerFlowSlice` unit test in `model_test.go` — `SDDMode != Multi` and cache-absent rows |
| 7 (INV-3) | `TestCustomPresetPostComponentFlowMatrix` golden |
| 8 (INV-4) | Existing ModelConfigMode exit-ramp test (unchanged) |
| INV-5 | Existing OpenCodePluginsStandalone guard test (unchanged) |
| Flow matrix | `TestPresetSelectionNextScreenFlowMatrix` golden (unchanged) |

---

## Out of Scope

- `shouldShow*` predicate logic — unchanged
- View/render code — no golden output changes
- `router.go` / `linearRoutes` — untouched
- Backups, uninstall, agent-builder, profiles, upgrade/sync nav graphs
