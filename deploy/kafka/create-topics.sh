#!/bin/sh
# Crea los tópicos como código. Es idempotente: se puede correr las veces que haga falta.
# Todo es configurable por variables de entorno (ver deploy/.env.example).
set -eu

BOOTSTRAP="${KAFKA_BOOTSTRAP_SERVERS:-kafka:9092}"
KT=/opt/kafka/bin/kafka-topics.sh

DAY_MS=86400000

# create <nombre> <particiones> [config=valor ...]
create() {
  name="$1"; partitions="$2"; shift 2
  configs=""
  for c in "$@"; do configs="$configs --config $c"; done
  # shellcheck disable=SC2086
  "$KT" --bootstrap-server "$BOOTSTRAP" --create --if-not-exists \
    --topic "$name" --partitions "$partitions" --replication-factor 1 $configs
}

# El orden por actor_id depende de la cantidad de particiones: si el tópico ya existe con otra
# cantidad, es un error (agregar particiones cambia la partición de cada actor_id).
assert_partitions() {
  name="$1"; expected="$2"
  actual=$("$KT" --bootstrap-server "$BOOTSTRAP" --describe --topic "$name" \
    | sed -n 's/.*PartitionCount: *\([0-9]*\).*/\1/p')
  if [ "$actual" != "$expected" ]; then
    echo "ERROR: $name tiene $actual particiones, se esperaban $expected. No se modifica." >&2
    echo "Para cambiarlo hay que borrar el tópico (y perder su contenido) a conciencia." >&2
    exit 1
  fi
}

EVENTS_PARTITIONS="${EVENTS_PARTITIONS:-12}"          # fijo desde el Bloque 2 (ADR-005)
ALERTS_PARTITIONS="${ALERTS_PARTITIONS:-12}"
ACTOR_RISK_PARTITIONS="${ACTOR_RISK_PARTITIONS:-12}"  # misma clave que events: misma partición
DLQ_PARTITIONS="${DLQ_PARTITIONS:-3}"
POLICIES_PARTITIONS="${POLICIES_PARTITIONS:-1}"

# Compactados: solo se compactan segmentos cerrados, por eso segment.ms chico en desarrollo.
COMPACT_SEGMENT_MS="${COMPACT_SEGMENT_MS:-60000}"

create events "$EVENTS_PARTITIONS" "retention.ms=$(( ${EVENTS_RETENTION_DAYS:-7} * DAY_MS ))"
create alerts "$ALERTS_PARTITIONS" "retention.ms=$(( ${ALERTS_RETENTION_DAYS:-14} * DAY_MS ))"
create dlq "$DLQ_PARTITIONS" "retention.ms=$(( ${DLQ_RETENTION_DAYS:-30} * DAY_MS ))"
create actor-risk "$ACTOR_RISK_PARTITIONS" \
  cleanup.policy=compact "segment.ms=$COMPACT_SEGMENT_MS" min.cleanable.dirty.ratio=0.01 delete.retention.ms=1000
create policies "$POLICIES_PARTITIONS" \
  cleanup.policy=compact "segment.ms=$COMPACT_SEGMENT_MS" min.cleanable.dirty.ratio=0.01 delete.retention.ms=1000

assert_partitions events "$EVENTS_PARTITIONS"
assert_partitions actor-risk "$ACTOR_RISK_PARTITIONS"

echo "Tópicos listos:"
"$KT" --bootstrap-server "$BOOTSTRAP" --list
