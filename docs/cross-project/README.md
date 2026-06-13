# Cross-Project-Notizen (Wayfinder ↔ Firefly)

Wayfinder (ASD-Frontend) und Firefly (Radar-Tracker) sind getrennte Projekte
mit getrennten Claude-Sitzungen. Solange beide Sitzungen keinen gemeinsamen
Repo-Zugriff haben, läuft der Austausch **manuell über diese Dateien**:

- **`todo-for-firefly.md`** — Beobachtungen/Wünsche aus der Wayfinder-Arbeit,
  die im Firefly-Projekt behandelt werden sollten (z.B. Schnittstellen-Lücken).
- **`todo-for-wayfinder.md`** — Beobachtungen/Wünsche aus dem Firefly-Projekt,
  die Wayfinder betreffen (vom Projektverantwortlichen hier eingefügt).

## Workflow

1. Eine Sitzung erkennt ein Cross-Project-Thema (z.B. "Firefly sollte X tun,
   damit Wayfinder Y kann").
2. Es wird als Eintrag in `todo-for-<anderes-projekt>.md` hier dokumentiert.
3. Der Projektverantwortliche trägt den Eintrag manuell ins andere Repo über
   (z.B. als GitHub Issue mit Label `from-wayfinder` / `from-firefly`).
4. Erledigte Einträge werden hier als ✅ markiert oder entfernt, sobald sie im
   Zielprojekt aufgenommen wurden.

## Zukünftige Automatisierung

Sobald eine Claude-Sitzung Zugriff auf **beide** Repos hat (siehe
[Claude Code on the web Docs](https://code.claude.com/docs/en/claude-code-on-the-web)),
kann dieser Austausch über GitHub Issues mit Cross-Repo-Links automatisiert
werden, statt über Dateien.
