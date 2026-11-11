  - Ziel: Plugins dürfen direkte, statische Routen definieren. Bei Kollision mit bestehenden Routen erfolgt automatische Namespacing.
  - Felder:
    - conflictPolicy?: "autoPrefix" | "error" (Default: "autoPrefix")
    - conflictTemplate?: string (nur relevant bei autoPrefix; Default: "/plugins/{pluginIdKebab}{route}")
      - Erlaubte Tokens: `{pluginId}`, `{pluginIdKebab}`, `{route}`
  - Ablauf:
    1) Prüfe Kollision (route + methods) gegen bereits registrierte Routen.
    2) Wenn keine Kollision: nutze `route` unverändert.
    3) Wenn Kollision und conflictPolicy = autoPrefix: bilde finale Route aus `conflictTemplate` durch Ersetzen der Tokens; Ergebnis muss ein statischer Pfad sein.
    4) Wenn Kollision und conflictPolicy = error: Validierung schlägt fehl.
  - Beispiel: pluginId = "samplePlugin", route = "/dashboard" → bei Kollision: "/plugins/sample-plugin/dashboard" (kebab‑Fall für pluginId)

Offene Punkte (abzustimmen)
- Offizielle Liste erlaubter Admin-UI-Slots.
- DB-Spezifikation: Details zu Typen/Naming/Regeln finalisieren (unten Draft enthalten).
- GraphQL/gRPC: Erweiterungen über die Base-Definition hinaus (Naming, CRUD‑Toggles, Services, Schemagenerierung).

Beispiele
- Werden unter example/plugin/ ergänzt: minimal, typisch gültig, komplex.

Validierung
- Pkl ist kanonisch. JSON-Validierung prüft die exportierte JSON-Repräsentation des Manifests gegen das JSON Schema.

GraphQL (Base v0.1)
- Ziel: Basis-Haken, um GraphQL-Unterstützung auf Ebene des Systems zu aktivieren. Details (Typ‑Mapping, CRUD‑Toggles, Feld‑Aliasse) folgen in künftigen Versionen.
- Struktur:
  - graphql?: {
      enabled?: boolean  // Default: false
    }
- Hinweise:
  - Bei `enabled = true` kann das System aus dem DB‑Schema (und Permissions) eine erste, minimale GraphQL‑Oberfläche generieren.
  - Erweiterungen (z. B. pro Tabelle toggles, Namenskonventionen) werden rückwärtskompatibel ergänzt.

gRPC (Base v0.1)
- Ziel: Interne Kommunikation zwischen Plugin und Govel‑Plugin‑System über gRPC. In v0.1 wird kein öffentliches API‑Exposure konfigurierbar gemacht; der Fokus liegt auf der sauberen Deklaration der Proto‑Quellen.
- Struktur:
  - grpc?: {
      enabled?: boolean,        // Default: false
      package?: string?,         // Optional; empfohlen: Standard aus pluginId ableiten (z. B. plugins.{pluginIdSnake}.v1)
      schemaName?: string?,      // Optional; Schema.org-Typ für Job-Workflows
      files?: string[],          // Liste plugin-relativer .proto-Dateien unter proto/**; Globs erlaubt (z. B. "proto/**/*.proto")
      importPaths?: string[],    // Zusätzliche Include-Pfade (plugin-relativ) für protoc -I
      includeBaseHealth?: boolean // Default: true – bindet Doc/bases/grpc/plugin_base.proto automatisch ein
    }
- Regeln & Konventionen:
  - Pfade sind plugin‑relativ und dürfen keine Parent‑Traversal enthalten (kein "..").
  - `grpc.files[]` muss auf Dateien unter `proto/` enden (Suffix `.proto`). Neben exakten Pfaden sind Globs zulässig (z. B. `proto/**/*.proto`).
  - `grpc.importPaths[]` sind Verzeichnisse (plugin‑relativ), z. B. `proto`, `third_party/google`.
  - Wenn `enabled = true` und weder `files` gesetzt sind noch `includeBaseHealth = true`, ist die Konfiguration ungültig (es gäbe dann keinen nutzbaren Descriptor).
  - Namenskonventionen (empfohlen, kann per `package` überschrieben werden):
    - package: `plugins.{pluginIdSnake}.v1`
    - option go_package: `plugins/{pluginIdSnake};{PluginIdPascal}V1`
- Code‑Generierung:
  - In v0.1 wird Codegen nicht im Manifest konfiguriert. Das Govel‑System generiert bei Bedarf Go‑Stubs oder nutzt dynamische gRPC‑Aufrufe (grpcdynamic). Spätere Versionen können optionale Codegen‑Ziele ergänzen.
- Beispiel (Ausschnitt):
```
"grpc": {
  "enabled": true,
  "package": "plugins.sample_plugin.v1",
  "schemaName": "JobPosting",
  "files": ["proto/health.proto", "proto/my_service.proto"],
  "importPaths": ["proto"],
  "includeBaseHealth": true
}
```

Status
- gRPC Base v0.1: STABLE. Felder/Regeln oben sind festgelegt; spätere Erweiterungen bleiben rückwärtskompatibel (z. B. zusätzliche optionale Felder für Code‑Generierung oder Service‑Exposure).

DB (PostgreSQL ≥ 13.4)
- Ziel: Das Manifest beschreibt Schema-Objekte deklarativ. Zusätzlich können SQL-Dateien (z. B. Seed-Daten oder komplexe Migrationen) referenziert werden. Migrations sind optional; Standard ist die direkte Schema‑Definition im Manifest.

Top-Level unter `db`
- engine: "postgres" (fix)
- minVersion?: string (Default "13.4")
- namespacePolicy: "required" | "optional" (Default: "required") – Tabellennamen müssen bei "required" mit `{pluginIdKebab}_` beginnen
- schema?: { tables: Table[] }
- indexes?: Index[] (global, alternativ pro Tabelle)
- constraints?: Constraint[] (global, alternativ pro Tabelle)
- permissions?: DbPermissionsDefaults? – Standardberechtigungen für Tabellen/Spalten
- sql?:
  - migrations?: SqlFile[]  (optional; z. B. Spezialfälle)
  - seed?: SqlFile[]        (optional; Start-/Demo-Daten)

Table
- name: string (snake_case)
- description?: string
- columns: Column[]
- primaryKey?: { columns: string[]; name?: string }
- indexes?: Index[]
- constraints?: Constraint[]
- permissions?: Permissions? (siehe Berechtigungsmodell)

Column
- name: string (snake_case)
- type: string (Postgres-Typ, z. B. "text", "varchar(255)", "uuid", "int4", "int8", "numeric(10,2)", "bool", "timestamp with time zone")
- nullable: boolean
- default?: string (als SQL-Fragment, z. B. "now()", "gen_random_uuid()")
- unique?: boolean (Kurzform für eindeutigen Index)
- references?: { table: string; column: string; onDelete?: Action; onUpdate?: Action; deferrable?: boolean; initiallyDeferred?: boolean }
- permissions?: Permissions? (siehe Berechtigungsmodell)

Index
- name: string (snake_case)
- table?: string (wenn global definiert)
- columns: (string | { name: string; op?: string })[]  (op für Ausdrucks-/Operator‑Klassen)
- unique?: boolean
- method?: "btree" | "gin" | "gist" | "brin"
- where?: string (Partial Index Bedingung)

Constraint
- name: string (snake_case)
- table?: string (wenn global definiert)
- type: "check" | "foreignKey" | "unique"
- expression?: string (für CHECK)
- columns?: string[] (für UNIQUE)
- references?: wie bei Column.references (für FK)

Action (FK)
- "no action" | "restrict" | "cascade" | "set null" | "set default"

SqlFile
- path: string (Dateipfad relativ zum Plugin‑Root; es dürfen auch Globs wie "sql/migrations/*.sql" verwendet werden)
- phase?: "pre" | "post" (Standard: "post") – Ausführungszeitpunkt relativ zur Schema‑Anwendung

Namenskonventionen & Reservierte Namen (DB)
- Tabellen- und Spaltennamen: snake_case, Regex: ^[a-z][a-z0-9_]*$
- Pflicht‑Namensraum für Tabellen: `{pluginIdSnake}_` (z. B. pluginId = "samplePlugin" → Präfix "sample_plugin_"). Der Validator erzwingt dies, sofern `db.namespacePolicy = "required"` (Default).
- Reservierte Spaltennamen, falls verwendet, sollten Standardbedeutungen haben:
  - id (Primärschlüssel, UUID oder BIGINT)
  - created_at, updated_at (Zeitstempel)
- Indizes und Constraints sollten eindeutig innerhalb eines Schemas benannt werden; Empfehlung: `{table}__{feld(e)}__{suffix}` (z. B. "orders__user_id__fk").

Registrierung im System
- Das System registriert alle im Manifest definierten Tabellen und Spalten inklusive Permissions in einer zentralen Registry.
- Plugins dürfen ausschließlich eigene, namespacete Tabellen definieren; Änderungen an Core‑Tabellen erfolgen nicht über das Plugin‑Manifest.
- WICHTIG (FK‑Beziehungen): Relationen sind strikt innerhalb eines Plugins zu halten. Fremdschlüssel auf Core‑Tabellen oder Tabellen anderer Plugins sind NICHT erlaubt. FK‑Referenzen müssen auf Tabellen zeigen, deren Name mit `{pluginIdSnake}_` beginnt.

Berechtigungsmodell (festgelegt)
- Ziel: Pro Tabelle/Spalte deklarativ Berechtigungen angeben, die das System zur Durchsetzung (UI, API) nutzt. Werte sind später in der Admin‑UI änderbar.
- Permissions‑Struktur (pro Objekt):
  - Permissions = {
      read?: string[],
      create?: string[],
      update?: string[],
      delete?: string[],
      manage?: string[]
    }
  - Jede Liste enthält Permission‑Keys (Strings). Bedeutung: Alle in der Liste genannten Keys sind für die jeweilige Operation erforderlich (UND‑Verknüpfung). Wenn die Liste leer ist, sind für diese Operation keine speziellen Berechtigungen erforderlich. Wenn das Feld fehlt, greift Vererbung (siehe unten).
- Globale Defaults:
  - DbPermissionsDefaults = { table?: Permissions, column?: Permissions }
    - Wirken als Basis; spezifische Einträge an Table/Column überschreiben diese Defaults.
- Vererbungs-/Override‑Regeln (pro Operation):
  1) Column.permissions.operation (falls gesetzt) OVERRIDES
  2) sonst Table.permissions.operation (falls gesetzt) OVERRIDES
  3) sonst db.permissions.column (für Spalten) bzw. db.permissions.table (für Tabellen) (falls gesetzt) OVERRIDES
  4) sonst Default‑Policy (siehe unten)
- Hinweis zur Auswertung:
  - „OVERRIDES“ bedeutet: die obere Ebene ersetzt die untere vollständig (keine Union/Merge). Damit reicht eine Definition an der Spalte, um die Tabellendefinition zu ersetzen.
- Konvention:
  - Konsistente Permission‑Keys über das System hinweg; Plugins liefern nur Keys, keine Rollenzuordnung.
- Default‑Policy (festgelegt):
  - read: erlaubt (wenn keine expliziten Permissions gesetzt sind)
  - create/update/delete: verboten (wenn keine expliziten Permissions gesetzt sind)
  - manage: erlaubt nur für Rollen mit dem reservierten Permission‑Key „admin“ (siehe unten). Hinweis: „superuser“ enthält ebenfalls „admin“, daher ist manage auch für Superuser erlaubt.

Reservierte Permission‑Keys (vom System bereitgestellt)
- admin: Kennzeichnet Admin‑Rechte. Dieser Key ist allen Admins und Superusern zugeordnet.
- superuser: Kennzeichnet Superuser‑Rechte. Dieser Key ist ausschließlich Superusern zugeordnet.

Hinweise
- Durch die Zuordnung „superuser enthält admin“ genügt z. B. `permissions.manage = ["admin"]`, damit sowohl Admins als auch Superuser die Operation ausführen dürfen.
- Wenn ein Plugin explizit nur Superuser zulassen möchte, kann es `permissions.manage = ["superuser"]` setzen.

Anmerkungen
- Das deklarative Schema (Pkl) ist die Quelle der Wahrheit. SQL‑Dateien sind ergänzend für Daten (Seed) oder Spezialfälle (z. B. komplexe Indexe/Features), die nicht elegant deklarativ abbildbar sind.
- Die Ausführung von SQL‑Dateien erfolgt in deklarierter Reihenfolge der Liste (Globs werden in lexikographischer Reihenfolge expandiert).

Pfad-Regeln & Packaging (Plugins)
- Grundsatz: Alle Pfade, die ein Plugin im Manifest angibt (Assets, Templates, SQL, etc.), sind PLUGIN‑RELATIV und plugin‑spezifisch. Absolute Pfade (beginnend mit "/") sind nicht erlaubt.
- Sicherheit/Isolation: Pfade dürfen keine Parent‑Traversal enthalten (keine ".." Segmente). Erlaubt sind nur Pfade innerhalb des Plugin‑Pakets.
- Beispiel‑Struktur (Empfehlung, nicht zwingend):
  - assets/ (Web‑Assets für Frontend/Admin‑UI)
    - frontend/... (Bundles, CSS, Bilder für Views)
    - admin/...    (Bundles, CSS für Admin‑Widgets)
  - views/...      (Templates des Plugins, z. B. Jet‑Templates)
  - sql/
    - migrations/...  (optionale Migrations)
    - seed/...        (Seed‑Daten)
- Installation/Deployment: Das System liest das Plugin, erzeugt systemweite Zielpfade und kopiert Dateien dorthin.
  - Ziel (Vorschlag, bitte bestätigen):
    - Web‑Assets → /public/plugins/{pluginIdKebab}/...
    - Templates  → app/views/plugins/{pluginIdSnake}/...
    - SQL bleibt paketintern und wird zur Laufzeit geladen/ausgeführt (kein Kopieren erforderlich)
- Rewriting/Referenzen: Beim Installieren werden referenzierte Pfade auf die erzeugten Zielpfade umgeschrieben. Tools verwenden anschließend die systemweiten Ziele (z. B. für HTTP‑Auslieferung der Assets oder Template‑Resolver).
- Core‑Assets/Templates: Referenzen auf System‑/Core‑Dateien sollen nicht als Pfad erscheinen. Falls notwendig, wird dafür eine gesonderte Alias‑Mechanik definiert (später). Für v0.1 referenzieren Plugins ausschließlich eigene Dateien.

## Datenbank (db)

Ziel: Deklarative Beschreibung der Plugin‑Tabellen, Indizes und Constraints. Der SQL‑Generator rendert daraus idempotente DDL für PostgreSQL (≥ 13.4). Tabellen werden automatisch mit einem Namensraumpräfix versehen: `plugin_<pluginId>_<tablename>`.

### Schema
- `db.schema.tables[]` (Pflicht, wenn `db` gesetzt ist)
  - `name`: string – Tabellenname ohne Präfix
  - `description?`: string
  - `columns[]`:
    - `name`: string
    - `type`: string – kanonische Typen (z. B. `string`, `int`, `bigint`, `decimal`, `float`, `bool`, `uuid`, `timestamp`, `date`, `time`, `json`, `bytes`, `enum`, `reference`, `array`)
    - `nullable`: bool
    - `length?`: int (für `string`)
    - `precision?`, `scale?`: int (für `decimal`)
    - `default?`: string – konstante Literale oder Ausdrücke (z. B. `now()`, `current_timestamp`)
    - `unique?`: bool – erzeugt eine Tabellen‑UNIQUE‑Klausel auf dieser Spalte
    - `references?`:
      - `table`: string – Zieltabelle (ohne Präfix)
      - `column`: string – Zielspalte
      - `onDelete?`: string – `NO ACTION|RESTRICT|CASCADE|SET NULL|SET DEFAULT`
      - `onUpdate?`: string – wie oben
  - `primaryKey?`:
    - `columns[]`: string – eine oder mehrere Spalten (Multi‑Column PK)
  - `indexes?`: `index[]` – optionale, tabellenlokale Index‑Definitionen (siehe Indexe)
  - `constraints?`: `constraint[]` – optionale, tabellenlokale Constraints (siehe Constraints)

### Indexe
- Auf Tabellenebene (`table.indexes[]`) oder global (`db.indexes[]`) definierbar. Global deklarierte Indexe benötigen das Feld `table`.
- Struktur `index`:
  - `name`: string – technischer Indexname (eindeutig innerhalb des Plugins)
  - `table?`: string – nur bei globaler Definition erforderlich
  - `columns[]`: string – mindestens eine Spalte
  - `unique?`: bool – erzeugt `CREATE UNIQUE INDEX`
  - `method?`: string – z. B. `btree`, `gin`
  - `where?`: string – partielle Indexbedingung, z. B. `deleted_at IS NULL`

Rendert zu idempotenten Statements der Form:
`CREATE [UNIQUE] INDEX IF NOT EXISTS "<name>" ON "plugin_<pluginId>_<table>" [USING <method>] (<columns>) [WHERE (<expr>)];`

### Constraints
- Auf Tabellenebene (`table.constraints[]`) oder global (`db.constraints[]`) definierbar. Global deklarierte Constraints benötigen das Feld `table`.
- Unterstützte Typen (`constraint.type`):
  - `unique` – benötigt `columns[]`
  - `check` – benötigt `expression`
  - `foreign` – benötigt `columns[]` und `references { table, column }`; optional `onDelete`, `onUpdate`

Bei Spalten kann `unique: true` gesetzt werden. Der Generator erzeugt in diesem Fall eine Tabellen‑UNIQUE‑Klausel für die einzelne Spalte. Für Mehrspalten‑UNIQUE bitte einen `constraint` vom Typ `unique` verwenden.

### Namensraum
- Alle generierten Objekte werden unter Verwendung des Plugin‑Namensraums erzeugt:
  - Tabellen: `plugin_<pluginId>_<name>`
  - Indizes/Constraints: wie angegeben, jedoch bezogen auf die namenspäfixierte Tabelle

### Validierungsregeln (CI/Runtime‑Checks)
Vor der SQL‑Generierung wird das Manifest geprüft. Fehlende/inkonsistente Definitionen führen zu einem Validierungsfehler:
- Tabellen
  - Keine doppelten Tabellennamen
  - Keine doppelten Spaltennamen je Tabelle
  - Falls `primaryKey` gesetzt: jede referenzierte Spalte muss existieren
- Indizes
  - `table` (bei globaler Definition) muss existieren
  - `columns[]` darf nicht leer sein und jede Spalte muss in der Tabelle existieren
- Constraints
  - `type` muss einer der unterstützten Typen sein (`unique`, `check`, `foreign`)
  - `unique`: `columns[]` muss gesetzt sein und Spalten müssen existieren
  - `check`: `expression` muss gesetzt sein
  - `foreign`: `columns[]` und `references.table`/`references.column` müssen gesetzt sein; die Spalten der Quelltabelle müssen existieren

Hinweis: Cross‑Plugin‑Referenzen werden syntaktisch akzeptiert, aber nicht global auf Existenz geprüft. Die zielseitige Tabelle sollte durch die Ziel‑Plugin‑Migration vorhanden sein.

### Zusätzliche SQL‑Dateien
- Optional können über `db.sql` zusätzliche SQL‑Skripte referenziert werden (z. B. Seeds, Erweiterungen). Diese werden transaktional zusammen mit den generierten DDL‑Statements ausgeführt.

### Laufzeit/Idempotenz
- Der Generator erzeugt `CREATE TABLE IF NOT EXISTS` und `CREATE INDEX IF NOT EXISTS` (bei Constraints wird idempotentes `ALTER TABLE ADD CONSTRAINT` durch einen Schutzblock erreicht). Dadurch sind wiederholte Deployments robuster.
