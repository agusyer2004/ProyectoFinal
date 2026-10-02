# ADR-001: Monorepo durante el desarrollo

- Estado: aceptada
- Fecha: 2026-10-01

## Contexto

La propuesta promete repositorios separados como entregable. En la Fase I, el esquema de eventos se comparte entre Go (Gateway, sink) y Python (simulador, luego scorer), y Docker Compose orquesta todo el sistema.

## Decisión

Un único repositorio con una carpeta por componente (`schema/`, `gateway/`, `sink/`, `simulator/`, `deploy/`, `migrations/`, `docs/`). Cada módulo Go y el proyecto Python tienen su propio manifiesto de dependencias.

## Consecuencias

- Un cambio de contrato y sus consumidores van en un mismo commit; no hay que sincronizar versiones a mano.
- El CI corre todo junto.
- Para la entrega final se puede dividir el repositorio por carpeta (por ejemplo con `git subtree split`) sin reescribir el diseño.
