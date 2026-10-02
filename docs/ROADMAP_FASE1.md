# Anomalous – Roadmap Fase I

Oct 1, 2026 · @agusyer

## Cómo usar este roadmap

La regla de orden es una sola: **que un evento atraviese todo el sistema lo antes posible, y recién después engordar cada pieza**. En la semana 5 tendría que existir un `curl` que termina como fila en TimescaleDB; todo lo demás de la Fase I es endurecer ese camino, medirlo y alimentarlo con el simulador.

Principios que justifican el orden:

- **Contrato primero.** El JSON Schema del evento lo tocan Gateway, sink, simulador y (después) scorer. Si cambia tarde, cambia en cuatro lugares.
- **Esqueleto caminante antes que features.** Un Gateway que responde siempre `allow` pero publica bien vale más que reglas de velocidad sin bus.
- **Medir desde temprano.** La tesis se defiende con métricas; si Prometheus llega al final, no hay historia de cómo evolucionó el sistema.
- **Cada bloque cierra con algo verificable.** Un test, un dashboard o un comando que se puede mostrar en la reunión quincenal.
- **Decisiones como ADRs.** Cada elección (cliente de Kafka, sink, monorepo) se anota en una página corta. Es material directo para la memoria.

Calendario estimado: semana 1 = 5 de octubre de 2026; 20 semanas de trabajo más 2 de margen por fiestas y vacaciones, cierre de fase hacia principios de marzo de 2027. La propuesta ubicaba la Fase I en agosto–diciembre de 2026, así que arrancar ahora corre la fase unos dos meses.

&#91;embedded content: Fase I · 8 bloques y 2 hitos por semana\]

Los bloques se solapan a propósito: la observabilidad arranca mientras se cierra el sink, y el simulador mientras se termina el dashboard.

## Bloque 0 – Fundaciones (semana 1)

Objetivo: un repo donde `make up` y `make test` funcionen desde el primer día, aunque no hagan casi nada.

- [x] **0.1 Monorepo durante el desarrollo.** La propuesta promete repos separados como entregable, pero en Fase I el esquema se comparte entre Go y Python y el Compose orquesta todo. Un monorepo evita sincronizar versiones a mano; se puede partir al final.
- [x] **0.2 Estructura de carpetas.** Sugerida: `schema/` (JSON Schema + fixtures), `gateway/` (Go), `sink/` (Go), `simulator/` (Python), `deploy/` (Compose, configs de Kafka, Prometheus, Grafana), `migrations/` (SQL), `docs/adr/`, `docs/contracts/`.
- [x] **0.3 Tooling.** Go con módulos y `golangci-lint`; Python con `uv` y `ruff`; un `Makefile` raíz con `up`, `down`, `test`, `lint`, `topics`, `migrate`. Pre-commit opcional.
- [ ] **0.4 CI mínimo.** GitHub Actions que corre lint y tests unitarios de Go y Python. Vacío pero verde: lo importante es que exista antes de que haya código.
- [x] **0.5 Primeros ADRs.** ADR-001 monorepo; ADR-002 Kafka con un único broker (ya está argumentado en la propuesta, solo pasarlo a formato ADR); ADR-003 cliente de Kafka en Go (ver 3.2).

**Listo cuando:** clonar el repo y correr `make test` da verde en tu máquina y en CI.

## Bloque 1 – Contratos (semanas 1 y 2)

Objetivo: el evento y la API del Gateway escritos y testeados antes de escribir el Gateway. Es el bloque que más retrabajo ahorra.

- [x] **1.1 JSON Schema v1 del evento.** Un sobre común más un `payload` por tipo, discriminado por `event_type` (`login_attempt`, `account_created`, `cart_action`, `transaction`, `read`, `transaction_outcome`). Campos del sobre:
  - `schema_version`, `event_id` (UUIDv7: ordenable por tiempo, útil para la hypertable), `event_type`, `trace_id`.
  - `actor_id` y `actor_id_source` (`user` | `device` | `ip`), para dejar explícita la regla de respaldo.
  - `occurred_at` (lo informa el cliente) y `received_at` (lo pone el Gateway). Son dos relojes distintos; no los mezcles.
  - `client_request_id`: identificador que manda el cliente. Sirve para unir el evento con la etiqueta de verdad del simulador (ver 1.2).
  - `decision` y `rules_fired` (en Fase I siempre `allow` y lista vacía, pero el campo ya existe).
  - Bloque `enrichment` vacío u opcional: GeoIP y huella llegan en Fase II, pero reservar el lugar evita una versión nueva del esquema.
- [x] **1.2 Las etiquetas de fraude NO viajan en el evento.** Si el campo `is_fraud` pasa por el Gateway, cualquier detector futuro podría leerlo por error y la evaluación queda contaminada. El simulador guarda las etiquetas en su propio log, indexadas por `client_request_id`, y la unión se hace al evaluar. Anotarlo como ADR: es un argumento metodológico para la tesis.
- [x] **1.3 Fixtures.** Un JSON válido por cada `event_type` y una batería de inválidos (campo faltante, tipo incorrecto, `event_type` desconocido). Son la base de los tests de contrato.
- [x] **1.4 Tests de contrato en ambos lenguajes.** Python con `jsonschema`; Go con un validador que soporte draft 2020-12 (por ejemplo `santhosh-tekuri/jsonschema`). Además del test de fixtures, un test de ida y vuelta: lo que serializan los structs de Go tiene que validar contra el esquema.
- [x] **1.5 Contrato HTTP del Gateway (OpenAPI).** `POST /v1/events` para operaciones y resultados (el resultado es otro `event_type`). Cabeceras: API key, firma HMAC, timestamp, nonce, `Idempotency-Key`. Respuesta: `decision`, `event_id`, `trace_id`, `reasons`. Códigos de error: 400 esquema inválido, 401 firma, 409 nonce repetido (aunque se implemente en Fase II), 503 no listo. También `GET /healthz` y `GET /readyz`.

**Listo cuando:** CI valida los fixtures en Go y en Python, y el OpenAPI renderiza sin errores.

## Bloque 2 – Infraestructura local (semanas 2 y 3)

Objetivo: `make up` levanta Kafka, las dos instancias de Redis y TimescaleDB, sanos y con los tópicos creados.

- [ ] **2.1 Kafka en KRaft, un nodo.** Imagen oficial `apache/kafka` (4.x), con el mismo proceso como broker y controller. Con un solo broker hay que bajar a 1 los factores de replicación internos (`offsets.topic.replication.factor`, `transaction.state.log.replication.factor`, `transaction.state.log.min.isr`, `min.insync.replicas`); si no, el productor idempotente falla de formas poco claras. Desactivar `auto.create.topics.enable`.
- [ ] **2.2 Dos listeners.** Uno interno para los contenedores y otro externo para depurar desde tu máquina. El simulador habla HTTP con el Gateway, no con Kafka, así que el listener externo es solo para debugging.
- [ ] **2.3 Creación de tópicos como código.** Un contenedor de inicialización que corre `kafka-topics.sh` y termina:
  - `events`: elegir las particiones ahora (por ejemplo 12, divisible por 1, 2, 3, 4 y 6 procesos de scorer). Agregar particiones después cambia qué partición le toca a cada `actor_id` y rompe el orden por actor.
  - `alerts`, `dlq` (retención más larga).
  - `actor-risk` y `policies` con `cleanup.policy=compact`. Aunque en Fase I nadie los use, crearlos ya permite probar tombstones a mano.
- [ ] **2.4 Redis A (contadores y perfiles).** `maxmemory` explícito y `maxmemory-policy volatile-ttl`, tal como dice la propuesta.
- [ ] **2.5 Redis B (control).** `maxmemory-policy noeviction` y AOF activado: si se reinicia y pierde los nonces, se abre una ventana de replay del tamaño de la ventana de timestamp.
- [ ] **2.6 TimescaleDB.** Imagen `timescale/timescaledb` con PostgreSQL reciente, volumen persistente, usuario por servicio (el sink no necesita los mismos permisos que el panel).
- [ ] **2.7 Healthchecks y orden de arranque.** `depends_on` con `condition: service_healthy`; sin esto, el sink arranca antes que la base y se cae en loop.
- [ ] **2.8 Perfiles de Compose.** `core` (lo de arriba), `obs` (Bloque 5) y `dev` (una UI de Kafka como kafbat/kafka-ui para mirar tópicos). Así el núcleo levanta liviano.

**Listo cuando:** `make up && make topics` deja todo `healthy` y podés producir y consumir un mensaje a mano en `events`.

## Bloque 3 – Esqueleto caminante (semanas 3 a 5)

Objetivo: un evento entra por HTTP y termina como fila en TimescaleDB. Todo feo, todo mínimo, pero de punta a punta. Este es el hito interno más importante de la fase.

- [ ] **3.1 Migraciones de TimescaleDB.** Con una herramienta versionada (por ejemplo `golang-migrate`), no con scripts sueltos.
  - `events` como hypertable particionada por `received_at`. Columnas extraídas para consultar (`event_id`, `event_type`, `actor_id`, `decision`, `trace_id`, `client_request_id`) y el evento completo en `JSONB`.
  - Clave única `(event_id, received_at)`: en una hypertable toda restricción única tiene que incluir la columna de tiempo (ver Trampas).
  - Índice `(actor_id, received_at DESC)`, que es el patrón de lectura del scorer y del panel.
  - Tablas `alerts` y `dlq` ya creadas, aunque queden vacías en esta fase.
- [ ] **3.2 Gateway mínimo en Go.** `POST /v1/events` que valida contra el esquema, asigna `event_id`, `received_at` y `trace_id`, responde siempre `allow` y publica en `events` con clave `actor_id`. Cliente de Kafka recomendado: **franz-go**. Es Go puro (sin cgo), trae el productor idempotente con `acks=all` por defecto y tiene `TryProduce`, que devuelve error en lugar de bloquear cuando el buffer está lleno: es exactamente el «buffer acotado con descarte contabilizado» de la propuesta. Documentarlo en ADR-003.
- [ ] **3.3 Sink mínimo.** Recomiendo empezar con un **consumer en Go** (franz-go + pgx) y no con Kafka Connect JDBC. El conector JDBC necesita el esquema embebido en cada mensaje o un Schema Registry, no maneja bien estructuras anidadas sin transformaciones, y su modo upsert hace `ON CONFLICT DO UPDATE`, no `DO NOTHING`. El consumer en Go son unas 200 líneas: lee un lote, hace un `INSERT … ON CONFLICT DO NOTHING` multi-fila y **commitea offsets solo después** del commit en la base. Si querés probar JDBC igual, ponele un límite de dos días.
- [ ] **3.4 Test de humo end-to-end.** Un script `make smoke` que hace `curl` al Gateway, espera unos segundos y verifica con `psql` que la fila existe con el mismo `event_id`.

**Listo cuando:** `make smoke` pasa. Es la primera demo para la dirección.

## Bloque 4 – Endurecer ingesta y sink (semanas 6 a 9)

Objetivo: que el camino del Bloque 3 cumpla las garantías que promete la propuesta (al menos una vez, descarte medido, escritura idempotente) y que haya tests que lo demuestren.

- [ ] **4.1 Productor con buffer acotado.** `TryProduce` con un máximo de registros en buffer configurable; si falla, se incrementa `events_dropped_total` con una etiqueta de motivo y la respuesta HTTP sale igual. La publicación nunca bloquea la decisión.
- [ ] **4.2 Apagado ordenado.** Ante `SIGTERM`, dejar de aceptar requests, hacer `Flush` del productor con un timeout y recién entonces salir. Sin esto, cada `docker compose restart` pierde eventos y contamina las mediciones.
- [ ] **4.3 Regla de `actor_id`.** Implementar el respaldo usuario → dispositivo → IP y registrar en `actor_id_source` cuál se usó. Test unitario por cada caso.
- [ ] **4.4 Autenticación del cliente.** API key más firma HMAC-SHA256 sobre método, ruta, timestamp, nonce y hash del cuerpo; ventana de timestamp configurable; comparación en tiempo constante (`hmac.Equal`). La unicidad del nonce en Redis B queda para Fase II, pero la firma conviene ahora: es barata y el simulador tiene que firmar desde el principio.
- [ ] **4.5 Evento de resultado.** `transaction_outcome` por el mismo endpoint, con referencia al evento original (`client_request_id` o `event_id`). Validar que el esquema obligue a esa referencia.
- [ ] **4.6 Readiness como mecanismo extensible.** `/readyz` consulta una lista de condiciones registradas. En Fase I la única es «productor conectado a Kafka»; en Fase II se agregan «actor-risk cargado hasta el end-offset» y «policies cargado». Diseñar el gancho ahora evita reescribir el arranque después.
- [ ] **4.7 Sink robusto.** Mensaje que no parsea → fila en la tabla `dlq` con la causa, y se sigue consumiendo. Base caída → reintentos con backoff y consumo pausado, sin commitear offsets. Consumir también los tópicos `alerts` y `dlq`.
- [ ] **4.8 Tests de integración con testcontainers-go.** Tres casos mínimos: camino feliz Gateway → Kafka → sink → base; mismo lote entregado dos veces da la misma cantidad de filas; sink matado a mitad de un lote y reiniciado no pierde ni duplica. Sumarlos al CI.

**Listo cuando:** los tres tests de integración corren en CI y podés reiniciar cualquier contenedor durante una carga sin perder eventos fuera de los contabilizados.

## Bloque 5 – Observabilidad y trazas (semanas 9 a 12)

Objetivo: un dashboard de Grafana que muestre TPS, P99, descartes y lag por partición, y una traza que siga un evento del Gateway al sink. La propuesta pide observabilidad «desde la primera fase»: el endpoint `/metrics` del Gateway conviene agregarlo ya en el Bloque 3 (son diez líneas con `client_golang`); este bloque monta el stack completo.

- [ ] **5.1 Prometheus y Grafana como código.** Datasources y dashboards en JSON dentro del repo, cargados por provisioning. Nada configurado a mano en la UI: si no está en el repo, no es reproducible.
- [ ] **5.2 Métricas del Gateway.** `http_requests_total` por `event_type`, decisión y código; histograma de latencia de decisión con buckets finos entre 0,5 y 20 ms (los buckets por defecto son demasiado gruesos para medir un P99 de 10 ms); `events_dropped_total`; registros en buffer del productor; métricas del runtime de Go (memoria, goroutines).
- [ ] **5.3 Métricas de Kafka.** kafka-exporter para el lag por partición y por consumer group; JMX exporter como agente Java del broker. Métricas del sink: filas escritas, latencia por lote, mensajes enviados a `dlq`.
- [ ] **5.4 OpenTelemetry.** SDK en Go, un OTel Collector y un backend de trazas: Jaeger es lo más simple; Tempo se integra con Grafana y deja todo en una sola pantalla. El contexto viaja en los headers de Kafka con el formato W3C `traceparent` (franz-go tiene el plugin `kotel`); el sink lo extrae y crea un span hijo.
- [ ] **5.5 Muestreo de trazas.** A 1.000 TPS no conviene guardar todas. Muestreo por ratio basado en el padre (por ejemplo 10 %), consistente entre servicios. Los percentiles salen de las métricas, no de las trazas.
- [ ] **5.6 Dashboard «Fase I».** Un único tablero con TPS, P50/P99 de decisión, descartes, lag por partición de `events` para el grupo del sink y tasa de escritura. Es la pantalla de la demo de cierre.

**Listo cuando:** lanzás tráfico, ves los paneles moverse, y desde un `trace_id` guardado en la base encontrás la traza Gateway → Kafka → sink.

## Bloque 6 – Simulador v1 (semanas 12 a 16)

Objetivo: tráfico legítimo creíble, reproducible por semilla, grabado como archivo y reinyectable contra cualquier versión. Hasta acá, para generar carga en los Bloques 3 a 5 alcanza con un script de pocas líneas o una herramienta como `vegeta`.

- [ ] **6.1 Separar «qué pasa» de «cómo se envía».** El modelo de comportamiento genera un escenario en un archivo JSONL; un emisor aparte lo lee y lo manda al Gateway. Así toda corrida es un archivo antes de ser tráfico, y la grabación y el replay de la propuesta salen gratis: son la misma operación.
- [ ] **6.2 Población de actores.** N usuarios con perfil propio: país e IPs habituales, 1 a 3 dispositivos, horario típico, monto de gasto con distribución log-normal. Llegadas por proceso de Poisson modulado por una curva diaria. Sesiones con secuencia lógica: login → acciones de carrito → transacción → resultado. Generador aleatorio con semilla fija.
- [ ] **6.3 Catálogo.** Generar el catálogo de productos (con alguna promoción vigente) como archivo versionado. En Fase II se carga en el Gateway; ahora solo tiene que existir y que las transacciones normales usen sus precios.
- [ ] **6.4 Formato del escenario.** Cabecera con semilla, versión del simulador y parámetros; una línea por request con su desplazamiento temporal en ms y el cuerpo. Las etiquetas de verdad van en un archivo aparte, indexadas por `client_request_id` (ver 1.2). **La firma HMAC se calcula al enviar, no al generar:** un escenario grabado hace meses tendría timestamps fuera de la ventana y el Gateway lo rechazaría entero.
- [ ] **6.5 Emisor de carga abierta.** Cada request sale en su instante programado, sin esperar la respuesta del anterior. Si el emisor espera, cuando el Gateway se pone lento baja la carga solo y el P99 medido sale artificialmente bueno (omisión coordinada). Soportar velocidad ×1, ×N y «lo más rápido posible».
- [ ] **6.6 Verificar que el emisor llega a 1.000 TPS.** Python con `asyncio` y `httpx` o `aiohttp` probablemente alcance, pero hay que medirlo. Si no llega, usar varios procesos o reescribir solo el emisor en Go; el modelo de comportamiento sigue en Python.
- [ ] **6.7 Host separado y relojes.** Correr el emisor en otra máquina con chrony en ambas. Registrar la latencia vista por el cliente para compararla con la del Gateway.
- [ ] **6.8 Escenarios congelados.** Carpeta inmutable con un checksum por escenario registrado en el repo (Git LFS si pesan). El primero: «tráfico normal, 10 minutos, 1.000 TPS».

**Listo cuando:** `simulate generate --seed 42` produce el mismo archivo dos veces seguidas y `simulate replay` lo inyecta y aparece completo en la base.

## Bloque 7 – Línea base, documentación y cierre (semanas 17 a 22)

Objetivo: salir de la Fase I con números reales del sistema sin detección, que son el punto de comparación de todo lo que venga, y con la documentación que pide el hito.

- [ ] **7.1 Caracterizar el hardware.** CPU, RAM, disco y red de cada host, anotados en el repo. Todo número de rendimiento sin esto no se puede interpretar.
- [ ] **7.2 Primera medición de línea base.** El escenario normal congelado a carga creciente (por ejemplo 250, 500, 1.000 y 1.500 TPS): TPS sostenido, P50/P99 de decisión, descartes y lag del sink. Exportar los datos crudos además de capturas de Grafana. Cuando en Fase II se agreguen Redis y las reglas, se va a poder decir cuánto costó cada pieza.
- [ ] **7.3 Matriz de política de falla.** Pasar la tabla de la sección 6.4 a `docs/contracts/` y agregar una columna: qué prueba de caos de Fase II o IV valida cada fila. Así la matriz se convierte en una lista de tests pendientes.
- [ ] **7.4 Métricas objetivo refinadas.** Con la línea base y el hardware en la mano, confirmar o ajustar 1.000 TPS y P99 de 10 ms. La propuesta ya prevé este ajuste en la primera fase.
- [ ] **7.5 Estado del arte.** Ampliar la bibliografía, en paralelo con todo lo anterior (una o dos horas por semana, no un bloque al final).
- [ ] **7.6 README de despliegue.** De cero a sistema corriendo en una máquina limpia. Probarlo en otra máquina o en una VM.
- [ ] **7.7 Informe de cierre de fase** para la dirección, con la demo del dashboard y los ADRs acumulados.

**Listo cuando:** se cumple el checklist del hito, al final de este documento.

## Qué NO hacer en la Fase I

Con tantas piezas, la tentación es empezar varias a la vez. Estas quedan explícitamente para después, aunque parezcan rápidas:

- Reglas de velocidad, nonces, idempotencia y locks en Redis. Los contenedores de Redis existen, pero el Gateway todavía no los usa.
- Enriquecimiento (GeoIP, CIDR, huella) y caché de catálogo en el Gateway. Solo se reserva el lugar en el esquema.
- Consumo de `actor-risk` y `policies` en el Gateway. Solo se deja el gancho de readiness.
- Load shedding y autoprotección (#5). Es núcleo, pero se diseña junto con los contadores en Fase II.
- Cualquier línea del scorer, del batch, del panel o del servicio de notificaciones.
- Patrones de fraude en el simulador. Primero tráfico normal creíble; los ataques se diseñan y congelan en Fase II, antes de ajustar detectores.
- Optimización prematura. Si la línea base ya da 1.000 TPS, no hay nada que optimizar todavía; si no, la medición dice dónde.

## Trampas técnicas conocidas

Problemas que suelen costar días en este stack. Conviene tenerlos a la vista desde el primer bloque.

| Trampa | Síntoma | Qué hacer |
| --- | --- | --- |
| Restricción única en hypertable | `ON CONFLICT (event_id)` falla al crear el índice único | Clave `(event_id, received_at)`; `received_at` lo fija el Gateway, así que los reintentos traen el mismo valor |
| Factores de replicación con un solo broker | El productor idempotente o los consumer groups dan errores de coordinador | Bajar a 1 los factores internos y `min.insync.replicas` (Bloque 2.1) |
| Particiones de `events` agregadas tarde | Eventos del mismo actor caen en otra partición y se pierde el orden | Fijar la cantidad en el Bloque 2 y no tocarla |
| Commit automático de offsets en el sink | Un crash pierde eventos ya commiteados pero no escritos | Commit manual, solo después del commit en la base |
| Escenarios firmados al generarse | El replay de un escenario viejo da 401 en todas las requests | Firmar en el emisor, en el momento del envío |
| Emisor que espera cada respuesta | P99 artificialmente bajo cuando el sistema se satura | Carga abierta con instantes programados (Bloque 6.5) |
| Mezclar `occurred_at` y `received_at` | Latencias negativas o absurdas | Latencias internas desde las trazas; `occurred_at` solo como dato del cliente |
| Buckets de histograma por defecto | Un P99 de 3 ms y uno de 9 ms se ven iguales | Buckets propios entre 0,5 y 20 ms |
| Compactación que «no anda» en pruebas | Los tombstones no borran la clave | Solo se compactan segmentos cerrados: en desarrollo, `segment.ms` chico |
| Medir con Docker Desktop en Mac o Windows | Números pobres e inestables por la VM intermedia | Las mediciones que van a la tesis, en un host Linux |

## Checklist del hito de cierre de Fase I

El hito de la propuesta es «ingesta funcional end-to-end con eventos sintéticos publicados, persistidos y consultables, y métricas operativas y trazas visibles en Grafana». Traducido a casillas verificables:

- [ ] `make up` levanta todo en una máquina limpia siguiendo solo el README.
- [ ] JSON Schema v1 y OpenAPI publicados en el repo, con tests de contrato verdes en Go y Python.
- [ ] El Gateway valida, firma y publica; responde `allow` y nunca bloquea por Kafka.
- [ ] `events_dropped_total` existe, se ve en Grafana y es cero dentro de la capacidad nominal.
- [ ] El sink escribe de forma idempotente; el test de doble entrega y el de reinicio a mitad de lote pasan en CI.
- [ ] Dashboard con TPS, P50/P99, descartes y lag por partición.
- [ ] Una traza completa Gateway → Kafka → sink, encontrable desde el `trace_id` de la base.
- [ ] El simulador genera tráfico normal reproducible por semilla, desde otro host, y el replay de un escenario congelado funciona.
- [ ] Línea base medida a carga creciente, con hardware documentado.
- [ ] Matriz de falla y ADRs en `docs/`, informe de cierre enviado a la dirección.
