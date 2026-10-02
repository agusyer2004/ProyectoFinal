# ADR-005: Tópicos como código y 12 particiones para `events`

- Estado: aceptada
- Fecha: 2026-10-02

## Contexto

`events` se particiona por `actor_id` para garantizar orden por entidad (ATO, viaje imposible, condiciones de carrera). Kafka asigna la partición con un hash de la clave módulo la cantidad de particiones: agregar particiones después reasigna actores a otras particiones y rompe el orden de los eventos ya escritos frente a los nuevos.

## Decisión

- `events` se crea con **12 particiones** y no se modifica. 12 es divisible por 1, 2, 3, 4, 6 y 12 procesos de scorer, lo que deja repartir la carga de forma pareja al escalar.
- `alerts` y `actor-risk` también con 12 (misma clave, misma partición que `events`). `dlq` con 3 y `policies` con 1 (poco volumen, sin necesidad de orden por actor).
- `actor-risk` y `policies` con `cleanup.policy=compact`; `segment.ms` chico en desarrollo porque solo se compactan segmentos cerrados.
- Retenciones por defecto: `events` 7 días, `alerts` 14 días, `dlq` 30 días (más larga, para analizar fallas). Todo es configurable por variables de entorno.
- Los tópicos se crean con `deploy/kafka/create-topics.sh` desde un contenedor de un solo uso (`make topics`). `auto.create.topics.enable=false`.
- El script es idempotente y **falla** si `events` o `actor-risk` ya existen con otra cantidad de particiones, en lugar de modificarlos en silencio.

## Consecuencias

- Reproducible: un volumen nuevo más `make topics` deja el mismo estado.
- Cambiar las particiones de `events` exige borrar el tópico y perder su contenido; queda como decisión consciente y no como accidente.
- Con 12 particiones y un solo broker no hay ganancia de rendimiento por paralelismo de broker; el valor es dejar abierta la escala de consumidores.
