# Anomalous – Proyecto Final de Carrera (UNS)

Sistema distribuido orientado a eventos para detección y mitigación en tiempo real de anomalías y fraudes de lógica de negocio en flujos transaccionales.

Stack: Go (Gateway, sink), Python (simulador; luego scorer y batch), Kafka en modo KRaft con 1 broker, Redis x2, TimescaleDB, Docker Compose, Prometheus/Grafana/OpenTelemetry.



No agregues líneas Co-Authored-By ni atribución a Claude en commits ni PRs

## Contexto (leer solo lo que haga falta)

- Roadmap de trabajo actual: @docs/ROADMAP_FASE1.md
- Propuesta completa (decisiones de diseño, taxonomía de fraudes, matriz de falla): docs/Propuesta_Anomalous_v4.md
- Diagrama de contenedores: docs/DiagramaContenedoresPF_v4.drawio (XML pesado: no importarlo con @, consultarlo solo si hace falta)

## Estado actual

Fase I (investigación e infraestructura base). Seguir el roadmap en orden, un bloque por vez. Al terminar un ítem, tildarlo en `docs/ROADMAP_FASE1.md`.

## Reglas de trabajo

- Hacer solo lo que pide el bloque en curso. No implementar nada de la sección "Qué NO hacer en la Fase I" del roadmap (reglas de velocidad, nonces, locks, enriquecimiento, consumo de actor-risk/policies, scorer, batch, panel, notificaciones).
- Antes de escribir código de un bloque, resumir en pocas líneas qué se va a hacer y qué archivos se tocan.
- Cada decisión técnica relevante se registra como ADR corto en `docs/adr/` (formato: contexto, decisión, consecuencias).
- Todo lo configurable va por variables de entorno o archivos en `deploy/`; nada hardcodeado.
- Cada pieza nueva debe levantar con `make up` y tener al menos un test que corra con `make test`.
- Si algo del roadmap contradice la propuesta, avisar y preguntar antes de decidir.

## Decisiones de diseño que no se pueden olvidar

- **Gateway = API de decisión**, no proxy. El cliente informa cada operación antes de ejecutarla y el resultado después (`transaction_outcome`). Decisiones: allow, review, step_up, block.
- **Las etiquetas de fraude NUNCA viajan en el evento.** El simulador las guarda aparte, indexadas por `client_request_id`. Evita contaminar la evaluación.
- **Kafka**: `events` particionado por `actor_id`; la cantidad de particiones se fija en el Bloque 2 y no se cambia después. Productor idempotente con `acks=all`, buffer acotado y descarte contabilizado (`events_dropped_total`). La publicación nunca bloquea la decisión.
- **Entrega al menos una vez** con consumidores idempotentes: el sink escribe con `ON CONFLICT DO NOTHING` y commitea offsets solo después del commit en la base.
- **Dos relojes distintos**: `occurred_at` (lo informa el cliente) y `received_at` (lo pone el Gateway). Las latencias internas se miden con trazas, no con `occurred_at`.
- **Redis**: instancia A (contadores y perfiles, `volatile-ttl`) e instancia B (control: idempotencia, nonces, locks; `noeviction` + AOF).
- **Política de falla por regla**, no global (ver sección 6.4 de la propuesta).
- **Simulador**: el escenario se genera a un archivo JSONL reproducible por semilla; un emisor aparte lo envía con carga abierta (sin esperar respuestas). La firma HMAC se calcula al enviar, no al generar.
- **Decisión pendiente sobre el sink**: el roadmap recomienda un consumer en Go (franz-go + pgx) en lugar de Kafka Connect JDBC, que es la opción preferida de la propuesta. Confirmar con el autor antes de implementar el ítem 3.3.

## Estructura del repositorio (monorepo durante el desarrollo)

```
schema/       JSON Schema de eventos + fixtures
gateway/      Gateway en Go
sink/         Sink de persistencia en Go
simulator/    Simulador de tráfico en Python
deploy/       Docker Compose, configs de Kafka, Prometheus, Grafana
migrations/   Migraciones SQL de TimescaleDB
docs/         Roadmap, propuesta, diagrama, adr/, contracts/
```

## Convenciones

- Go: módulos, `golangci-lint`, tests con `go test`, integración con testcontainers-go.
- Python: `uv` para dependencias, `ruff` para lint, `pytest` para tests.
- `Makefile` en la raíz con los objetivos: `up`, `down`, `test`, `lint`, `topics`, `migrate`, `smoke`.
- Commits chicos, uno por ítem del roadmap cuando sea posible.
- Idioma: documentación y comentarios de diseño en español; nombres de código, métricas y campos en inglés.

## Mediciones

Los números que van a la tesis se toman en un host Linux (no Docker Desktop), con el hardware documentado y el simulador en otro host con relojes sincronizados (chrony).
