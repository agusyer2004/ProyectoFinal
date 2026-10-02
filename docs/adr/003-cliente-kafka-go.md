# ADR-003: Cliente de Kafka en Go

- Estado: propuesta (confirmar al implementar el ítem 3.2)
- Fecha: 2026-10-01

## Contexto

El Gateway publica en `events` sin bloquear nunca la decisión, con buffer acotado y descarte contabilizado. El sink consume con commit manual de offsets. Ambos necesitan un cliente Kafka en Go.

## Decisión

Usar **franz-go**:

- Go puro (sin cgo): imágenes y builds más simples que con librdkafka.
- Productor idempotente con `acks=all` por defecto.
- `TryProduce` devuelve error en lugar de bloquear cuando el buffer está lleno, lo que implementa directamente el «buffer acotado con descarte contabilizado».
- Plugin `kotel` para propagar el contexto de trazas en los headers.

## Consecuencias

- Una sola librería para Gateway y sink.
- Alternativas descartadas: `confluent-kafka-go` (cgo) y `segmentio/kafka-go` (sin productor idempotente).
- Si el rendimiento medido en la línea base fuera insuficiente, se revisa con datos.
