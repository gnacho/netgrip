---
type: planning
entity: todo
plan: "advanced-section"
updated: "2026-10-06"
---

# Todo: advanced-section

> Tracking [advanced-section](plan.md)

## Active Phase: 1 - Advanced mode + section framework

### Phase Context

- **Scope**: [Phase 1](phases/phase-1.md)
- **Implementation**: Not authored yet
- **Latest Handover**: None
- **Relevant Docs**: issues #441 (framework), #442 (SNMP), #443 (switch); `internal/modules/vlan.go` (existing VLAN probe/apply); `app/src/components/tools/advanced.tsx` (cards to move)

### In Progress

- [x] Worktree + branch `feat/441-advanced-mode` from main <!-- completed: 2026-10-06 -->
- [x] Mover LAG + tarjetas de ingeniería de Puertos (perfiles/modos/plantillas/estadísticas) a Avanzadas <!-- completed: 2026-10-06 -->

### Completed

### Blocked

## Changelog

### 2026-10-06

- Plan created with maintainer decisions (UCI flag + toggle, grouping, snmpd, DSA, UniFi style, 3 phases). Phase 1 populated.
- Phase 1 implementation on `feat/441-advanced-mode` (worktree `~/temp/opencode-work/netgrip-441`), 2 commits: framework + moves. Nota del mantenedor aplicada: reagrupar lo existente, no duplicar. Preview rt-lab `0.72.99-adv441c` con flag ON.
