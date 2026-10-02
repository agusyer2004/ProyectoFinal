# ADR-004: Las etiquetas de fraude no viajan en el evento

- Estado: aceptada
- Fecha: 2026-10-01

## Contexto

El simulador sabe cuáles de sus eventos son fraude. Si esa etiqueta (`is_fraud`, tipo de ataque, etc.) viajara en el evento por el Gateway y Kafka, cualquier detector actual o futuro (reglas, scorer, batch) podría leerla, por error o por conveniencia, y la evaluación de la tesis quedaría contaminada: las métricas de detección medirían la lectura de la respuesta, no la capacidad de detectar.

## Decisión

- El esquema de evento v1 no tiene ningún campo de etiqueta y rechaza propiedades desconocidas (`unevaluatedProperties: false`), tanto en el cuerpo que envía el cliente como en el evento persistido. Un evento con `is_fraud` es inválido y el Gateway responde 400.
- El simulador escribe las etiquetas en un archivo aparte, indexadas por `client_request_id`.
- La unión entre eventos y etiquetas se hace solo en la evaluación, fuera de la ruta de decisión, mediante `client_request_id`.
- Hay un fixture inválido y un test de contrato que fijan esta regla.

## Consecuencias

- La evaluación es metodológicamente limpia: ningún componente del sistema puede acceder a la verdad de terreno.
- `client_request_id` pasa a ser obligatorio y debe ser único por request; es la única llave de unión.
- Agregar un campo al contrato exige subir la versión del esquema o ampliar sus definiciones explícitamente; no se cuelan campos por accidente.
- Las etiquetas necesitan su propio formato y archivo en el simulador (Bloque 6.4).
