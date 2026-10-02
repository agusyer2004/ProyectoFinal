# ADR-002: Kafka en modo KRaft con un único broker

- Estado: aceptada
- Fecha: 2026-10-01

## Contexto

El sistema necesita un bus de eventos durable, particionado por `actor_id` y con semántica de al menos una vez. La propuesta (ver su sección de despliegue) argumenta que el alcance del proyecto y el hardware disponible no justifican un clúster.

## Decisión

Un solo nodo Kafka en modo KRaft (sin ZooKeeper), con el mismo proceso como broker y controller, imagen oficial `apache/kafka`. Los factores de replicación internos y `min.insync.replicas` se fijan en 1. La creación automática de tópicos se desactiva; los tópicos se crean como código.

## Consecuencias

- Despliegue simple y reproducible con Compose.
- No hay tolerancia a la caída del broker: se asume en el análisis de fallas y se documenta en la matriz de política de falla.
- `events` se crea con una cantidad fija de particiones (Bloque 2.3); agregarlas después rompe el orden por actor.
- Los resultados de rendimiento aplican a esta topología, no a un clúster replicado.
