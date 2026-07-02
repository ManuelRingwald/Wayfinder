# CI & Branch-Schutz — Wayfinder

Diese Datei erklärt, was die automatische Prüfung (CI) tut und wie `main`
geschützt wird. Sie ist bewusst einsteiger-freundlich.

## Was die CI tut

Die CI (`.github/workflows/ci.yml`) läuft **automatisch** bei jedem Push auf
`main` und bei jedem Pull Request gegen `main`, aufgeteilt in drei Jobs (damit
jeder einzeln als Pflicht-Check verlangt werden kann):

| Job | Prüft | Charter-Bezug (CLAUDE.md §6) |
|-----|-------|------------------------------|
| **Go (build · vet · fmt · test)** | `gofmt -l`, `go vet ./...`, `go build ./...`, `go test ./...` | Go-Gates |
| **golangci-lint** | `golangci-lint run` (Konfig: `.golangci.yml`) | Lint-Gate |
| **Frontend (test · build · dist)** | `npm ci`, `vitest`, `vite build`, **dist-Aktualitätscheck** | Frontend + eingebettetes Bundle |

**dist-Check (wichtig für dieses Repo):** Das Go-Binary bettet
`internal/webui/dist` ein. Wenn du das Frontend änderst, aber vergisst, das
Bundle neu zu bauen und mitzucommitten, würdest du eine **veraltete UI**
ausliefern. Die CI baut das Frontend neu und schlägt fehl, wenn sich das
committete `dist` unterscheidet. Fix lokal:

```bash
cd frontend && npm run build && cd .. && git add internal/webui/dist && git commit
```

### golangci-lint & errcheck
`.golangci.yml` nutzt das Standard-Linter-Set inkl. **errcheck** (prüft
ungeprüfte Fehler-Rückgaben). Die anfänglichen ~30 Alt-Befunde wurden in #124
aufgeräumt (echte Fehlerpfade behandelt, bewusste fire-and-forget-Aufrufe mit
`_ =` markiert), danach wurde errcheck scharf geschaltet.

## `main` schützen (einmalig im GitHub-Web-UI)

> Wichtig: Pflicht-Checks lassen sich erst verlangen, **nachdem die CI mindestens
> einmal gelaufen ist** (GitHub muss die Check-Namen einmal gesehen haben). Also:
> erst diesen CI-PR mergen, CI einmal laufen lassen, dann die Regel anlegen.

1. **Settings → Branches → Add branch ruleset** (oder „Add rule").
2. **Branch name pattern:** `main`.
3. Aktivieren:
   - ☑ **Require a pull request before merging** (kein direkter Push auf `main`).
     Approvals: **0** (Solo — du mergst deine eigenen PRs).
   - ☑ **Require status checks to pass before merging** →
     ☑ **Require branches to be up to date** → diese drei Checks auswählen:
     **`Go (build · vet · fmt · test)`**, **`golangci-lint`**,
     **`Frontend (test · build · dist)`**.
   - ☑ **Block force pushes** und **Restrict deletions**.
4. Speichern.

Damit ist die GitHub-Warnung „main is not protected" weg, und nichts landet mehr
ohne grüne CI auf `main`.

### Später verschärfen (wenn ein Team dazukommt)
- Approvals auf **1** (Vier-Augen-Prinzip).
- **Require conversation resolution** + ggf. **Require linear history**.
