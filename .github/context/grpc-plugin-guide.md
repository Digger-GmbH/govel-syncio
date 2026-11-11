# Leitfaden: gRPC‑Plugins entwickeln (Govel Plugin System)

Dieser Leitfaden zeigt Schritt für Schritt, wie du ein Plugin mit gRPC‑Schnittstelle für das Govel Plugin System entwirfst, baust, testest und integrierst. Ziel ist, dass du eigenständig ein lauffähiges gRPC‑Plugin liefern kannst.

Inhalte:
- Überblick & Anforderungen
- Ordnerstruktur & Manifest
- Proto‑Design & Namenskonventionen
- Code‑Generierung (Go)
- Server‑Implementierung (Go)
- Policies: Versionierung, Fehler, Zeitlimits, Retries, Circuit Breaking
- Testen mit grpcurl
- Integration in Govel (Registry & Admin)
- Akzeptanzkriterien & Checkliste
- Troubleshooting

Weitere Referenzen:
- Base‑Doku: `Doc/plugin/bases/grpc/README.md`
- Base‑Proto: `Doc/plugin/bases/grpc/plugin_base.proto`
- gRPC Betriebsrichtlinien: `Doc/grpc/RETRY_CONFIG.md`, `Doc/grpc/circuit_breaker.md`
- Beispiel‑Protos: `Doc/plugin/example/plugin/proto/*.proto`

---

## 1) Überblick & Anforderungen
- Programmiersprache: Go ≥ 1.23 (Repo nutzt moderne Go‑Idiome)
- Build: `protoc` mit `protoc-gen-go` und `protoc-gen-go-grpc`
- Kommunikationsstil: request/response, Streaming optional
- Ziel: Stabile, versionierte API je Plugin mit klaren Fehlercodes

Empfohlene Tooling‑Versionen (mindestens):
- protoc ≥ 3.21
- google.golang.org/protobuf ≥ v1.33
- google.golang.org/grpc ≥ v1.60

---

## 2) Ordnerstruktur & Manifest
Beispielhafte Pluginstruktur:
```
plugins/
  <pluginId>/
    manifest.json
    proto/
      <dein>.proto
    src/
      main.go
      ...
    data/            # optional (lokaler State)
```

Manifest‑Ausschnitt (`manifest.json`):
```json
{
  "schemaVersion": "1.0.0",
  "plugin": {
    "id": "samplePlugin",
    "version": "0.1.0",
    "authors": [{ "name": "Du" }]
  },
  "grpc": {
    "enabled": true,
    "package": "plugins.sample_plugin.v1",
    "schemaName": "Thing",
    "files": ["proto/**/*.proto"],
    "importPaths": ["proto"],
    "includeBaseHealth": true
  }
}
```
Hinweise:
- `plugin.id` in lowerCamelCase; `package` in snake_case mit Version (`v1`).
- `includeBaseHealth` bindet `PluginHealth` Service aus der Base ein.

---

## 3) Proto‑Design & Namenskonventionen
Siehe Base‑Doku unter `Doc/plugin/bases/grpc/README.md`. Kurzfassung:
- package: `plugins.{pluginIdSnake}.v1`
- option go_package: `plugins/{pluginIdSnake};{PluginIdPascal}V1`
- Stabilität: Eine veröffentlichte `v1`‑API ist abwärtskompatibel zu halten. Brechende Änderungen erfordern `v2`.

Minimalbeispiel (`proto/echo.proto`):
```proto
syntax = "proto3";

package plugins.sample_plugin.v1;
option go_package = "plugins/sample_plugin;SamplePluginV1";

import "Doc/plugin/bases/grpc/plugin_base.proto"; // optional, wenn nicht auto‑inkludiert

message EchoRequest { string text = 1; }
message EchoReply { string text = 1; }

service EchoService {
  rpc Echo(EchoRequest) returns (EchoReply);
}
```

Fehler‑ und Statusmodell:
- Bevorzuge Standard‑gRPC Statuscodes (OK, INVALID_ARGUMENT, NOT_FOUND, ALREADY_EXISTS, PERMISSION_DENIED, UNAUTHENTICATED, RESOURCE_EXHAUSTED, FAILED_PRECONDITION, ABORTED, OUT_OF_RANGE, UNIMPLEMENTED, INTERNAL, UNAVAILABLE, DEADLINE_EXCEEDED).
- Payload‑Fehlerdetails sind optional; nutze sie sparsam und konsistent.

---

## 4) Code‑Generierung (Go)
Voraussetzungen installieren:
```bash
# Generatoren installieren
GO111MODULE=on go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
GO111MODULE=on go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

Code generieren (im Pluginverzeichnis):
```bash
protoc \
  -I . \
  -I ../../.. \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  proto/**/*.proto
```
Erläuterung:
- `-I .` → dein Plugin‑Proto‑Pfad
- `-I ../../..` → Repo‑Root, damit `Doc/plugin/bases/grpc/plugin_base.proto` gefunden wird
- `paths=source_relative` generiert Dateien neben den Quellen

---

## 5) Server‑Implementierung (Go)
Minimaler gRPC‑Server (`src/main.go`):
```go
package main

import (
  "context"
  "log"
  "net"

  "google.golang.org/grpc"
  "google.golang.org/grpc/health"
  healthpb "google.golang.org/grpc/health/grpc_health_v1"

  pb "./proto" // passe Pfad/Modul an, z. B. plugins/sample_plugin
)

type echoServer struct{ pb.UnimplementedEchoServiceServer }

func (s *echoServer) Echo(ctx context.Context, req *pb.EchoRequest) (*pb.EchoReply, error) {
  if req.GetText() == "" {
    return nil, status.Error(codes.InvalidArgument, "text required")
  }
  return &pb.EchoReply{Text: req.GetText()}, nil
}

func main() {
  lis, err := net.Listen("tcp", ":50051")
  if err != nil { log.Fatalf("listen: %v", err) }

  s := grpc.NewServer()

  // Health (optional, wenn Base genutzt)
  hs := health.NewServer()
  healthpb.RegisterHealthServer(s, hs)

  pb.RegisterEchoServiceServer(s, &echoServer{})

  log.Println("gRPC Plugin Server listening on :50051")
  if err := s.Serve(lis); err != nil { log.Fatalf("serve: %v", err) }
}
```
Hinweise:
- Verwende Context‑Deadlines in Handlers (`ctx, cancel := context.WithTimeout(...)`).
- Antworte mit passenden gRPC Statuscodes (siehe Abschnitt Policies).

---

## 6) Policies (Verbindliche Richtlinien)
- Versionierung
  - Nutze semantische Versionierung im Proto‑Package: `...v1`.
  - Keine brechenden Änderungen in `v1`. Für Breaking Changes hebe auf `v2`.
- Zeitlimits
  - Serverseitig: setze sinnvolle Default‑Deadlines (z. B. 5–30s pro RPC) – insbesondere bei externen APIs.
  - Clientseitig: Fordere Deadlines ein; lehne „unbounded“ Requests ab, wenn sinnvoll.
- Retries
  - Nur bei idempotenten Operationen und transienten Fehlern (`UNAVAILABLE`, `DEADLINE_EXCEEDED`, `RESOURCE_EXHAUSTED`).
  - Siehe `Doc/grpc/RETRY_CONFIG.md` für Standard‑Backoff.
- Circuit Breaking
  - Aktiviere/unterstütze Circuit‑Breaker‑Policies (siehe `Doc/grpc/circuit_breaker.md`).
- Fehlercodes
  - Mapping gängiger Fälle: Validation → `INVALID_ARGUMENT`; Nicht gefunden → `NOT_FOUND`; Konflikte → `ALREADY_EXISTS/ABORTED`; externe Abhängigkeit down → `UNAVAILABLE`.
- Beobachtbarkeit
  - Logge Request‑IDs/Korrelation, z. B. aus Metadata. Sammle Metriken (Latenz, Fehlerraten) falls möglich.

---

## 7) Testen mit grpcurl
Starte deinen Server lokal (z. B. `:50051`) und rufe einen RPC auf:
```bash
grpcurl -plaintext -d '{"text":"hello"}' localhost:50051 plugins.sample_plugin.v1.EchoService/Echo
```
Erwartete Antwort:
```json
{ "text": "hello" }
```
Health‑Check (falls eingebunden):
```bash
grpcurl -plaintext -d '{}' localhost:50051 plugins.sample_plugin.v1.PluginHealth/Check
```

---

## 8) Integration in Govel
- Registry: Beim Installieren des Plugins werden gRPC‑Metadaten (Paket, Services, RPCs) in die Registry übernommen (siehe DB‑Migrationen mit `grpc_registry`).
- Admin‑UI: gRPC‑fähige Plugins werden im Adminbereich gekennzeichnet; Aufruf/Monitoring kann dort angebunden werden (systemabhängig).
- Manifest: Stelle sicher, dass alle `.proto`‑Dateien über `grpc.files` erreichbar und kompilierbar sind.

---

## 9) Akzeptanzkriterien & Checkliste
Erfolgreich, wenn ALLE Punkte erfüllt sind:
1. Manifest enthält gültigen `grpc`‑Block (enabled, package, files, optional includeBaseHealth).
2. Proto‑Schemata folgen Namens‑/Versionierungsregeln (`plugins.{pluginIdSnake}.v1`).
3. Go‑Code wurde aus Protos generiert; der Server kompiliert.
4. Mindestens ein RPC funktioniert lokal; `grpcurl` liefert erwartete Antwort.
5. Fehler werden mit passenden gRPC‑Statuscodes gemappt.
6. Zeitlimits/Retry‑Policy sind implementiert oder dokumentiert.
7. Plugin lässt sich im System installieren; Registry wird befüllt (ohne Fehler).

Kurz‑Check (Copy/Paste):
- [ ] `manifest.json` gRPC‑Block korrekt
- [ ] `protoc` läuft ohne Fehler
- [ ] Server startet auf Port `:50051`
- [ ] `grpcurl` Test erfolgreich
- [ ] Health‑Check antwortet (optional)
- [ ] Fehlercodes und Timeouts geprüft

---

## 10) Troubleshooting
- `protoc: file not found Doc/plugin/bases/grpc/plugin_base.proto`
  - Füge Repo‑Root als Include‑Pfad hinzu: `-I ../../..` (vom Pluginordner aus)
- `unimplemented service` oder `method not found`
  - Prüfe, ob die Server‑Implementierung registriert wurde (`Register...Server`).
- `UNAVAILABLE` / Verbindungsfehler
  - Läuft der Server auf dem erwarteten Port? Nutze `-plaintext` in grpcurl, wenn kein TLS.
- Namenskonflikte im Package
  - Achte auf konsistente `package` und `go_package` Optionen in allen Protos.

---

## Beispielreferenzen im Repo
- Facebook‑Marketplace Plugin (vollständig, inkl. gRPC & Tests): `app/plugins/gvlpluginfacebookmarketplace/`
- Mobile‑Client Protos: `app/plugins/gvlpluginmobile/proto/`
- Beispiel‑Protos (Shop/Warehouse): `Doc/plugin/example/plugin/proto/`
