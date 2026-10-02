**Universidad Nacional del Sur**

*Departamento de Ciencias e Ingeniería de la Computación*

**Propuesta de Proyecto Final de Carrera**

**Diseño e implementación de una arquitectura distribuida orientada a eventos para la observabilidad y detección de anomalías lógicas en flujos transaccionales**

*Proyecto «Anomalous»*

Alumno: [Nombre y Apellido] LU: [Número]

Director propuesto: [Nombre] Codirector propuesto: [Nombre]

*Fecha estimada de presentación: Febrero 2028*

*Bahía Blanca – 2026*

Versión final (v4)

**Índice**

- Resumen ejecutivo
- Contexto y definición del problema
- Estado del arte y posicionamiento
- Objetivos
- Alcance y exclusiones
- Marco tecnológico y arquitectura propuesta
- Taxonomía de fraudes y asignación a planos de detección
- Metodología de validación
- Plan de trabajo y cronograma
- Plan de gestión de riesgos
- Resultados esperados
- Solicitud a la dirección

Referencias preliminares

# 1. Resumen ejecutivo

El presente documento propone el desarrollo de un sistema distribuido, asincrónico y tolerante a fallos para la detección en tiempo real y la mitigación de anomalías operativas y fraudes de lógica de negocio en flujos transaccionales estructurados, aplicable al sector de tecnología financiera (FinTech) y a plataformas que procesan eventos de alta concurrencia.

La motivación parte de un gap concreto: las defensas convencionales basadas en reglas estáticas y firewalls perimetrales no detectan abusos que respetan los mecanismos de autenticación pero violan la semántica esperada del negocio. La solución propuesta combina una Arquitectura Orientada a Eventos (EDA), un motor de reglas de baja latencia y un plano de análisis estadístico no supervisado para abordar el problema en dos dimensiones temporales complementarias: respuesta sincrónica inmediata y análisis asincrónico contextual, unidas por un lazo de realimentación que traslada las detecciones tardías a las decisiones inmediatas.

Respecto de la concepción inicial, esta versión incorpora cinco decisiones arquitectónicas de fondo:

- **Plano analítico dividido en dos componentes.** Un **scorer de streaming** para anomalías puntuales evaluables evento a evento, y un **analizador batch** para patrones multi-evento y multi-entidad (triangulación, lavado, abuso de promociones), porque son cargas de trabajo, cadencias y modelos distintos.
- **Separación entre lo determinista y lo estadístico.** Lo validable con una regla (idempotencia, firma de request, precio contra catálogo, cantidades inválidas) se resuelve en la ruta sincrónica; lo estadístico, en los planos de streaming y batch. Cada control vive en la capa correcta.
- **Gateway como servicio de decisión.** El cliente informa cada operación antes de ejecutarla, recibe una decisión (allow, review, step_up o block) y comunica luego el resultado. La política de falla se define **por regla** y no de forma global: fail-open para los controles de velocidad, fail-closed o con fallback local para los controles deterministas que un atacante podría intentar degradar. La tensión entre baja latencia y no pérdida de eventos se resuelve de forma explícita: productor idempotente con acks=all, buffer acotado y descarte contabilizado.
- **Lazo de mitigación cerrado.** Las detecciones asincrónicas del scorer, del batch y del analista actualizan un estado de riesgo por actor (tópico compactado actor-risk) que el Gateway consulta en cada evento, con expiración y override manual. Sin este lazo el sistema solo observaría; con él, detecta y responde.
- **Responsabilidades separadas y una única fuente de verdad.** La persistencia histórica la realiza un consumer group dedicado e idempotente, no el scorer. Los contadores de velocidad tienen a Redis como única fuente de verdad, y la memoria local del Gateway queda como primera línea gruesa de autoprotección.

Esta versión incorpora además un conjunto de garantías operativas que endurecen el diseño en sus puntos más delicados (sección 6.4): protección del perfil caliente frente a rebalanceos del consumer group, separación de la durabilidad entre contadores y perfiles, scoring por micro-lotes, arranque seguro del Gateway, cierre del ciclo de las decisiones *review* y *step_up* con la señal de resultado del cliente, y un esquema de evaluación reproducible (trazabilidad distribuida, pruebas de caos y repetición de escenarios congelados).

El plazo estimado es de 18 meses, con un **núcleo mínimo defendible** identificado desde el inicio y un plan de gestión de riesgo explícito que prevé degradación controlada de componentes en caso de desvíos. El alcance por defecto es ambicioso: cubrir prácticamente todo el espectro de fraudes catalogado, repartido entre los tres planos de detección. El núcleo mínimo opera como piso de garantía académica, no como objetivo. La validación se realiza principalmente sobre un simulador de tráfico desarrollado en el marco del proyecto, con datasets sintéticos de fraude inyectado de forma etiquetada, escenarios de evaluación congelados antes del ajuste de los detectores y, como plan complementario, datasets públicos de referencia.

# 2. Contexto y definición del problema

## 2.1. Contexto del dominio

El fraude en plataformas FinTech y sistemas transaccionales ha evolucionado más allá de la clonación de credenciales o el robo de datos de tarjetas. Los atacantes modernos aprovechan vulnerabilidades en la lógica de negocio y técnicas de automatización avanzada para explotar APIs y sistemas de pago sin violar necesariamente las reglas de autenticación tradicionales. Ejemplos típicos incluyen abuso de campañas promocionales mediante creación masiva de cuentas, manipulación de flujos de devolución, explotación de condiciones de carrera en operaciones concurrentes y patrones de uso compatibles con bots que evaden mecanismos básicos de detección.

## 2.2. Planteo del problema

Las arquitecturas monolíticas y los sistemas de seguridad convencionales (firewalls perimetrales, WAFs basados en firmas y motores de reglas aislados) presentan limitaciones estructurales para abordar este tipo de amenazas:

- **Falta de correlación histórica:** evalúan transacciones de forma aislada, sin contextualizar contra el comportamiento previo del actor.
- **Acoplamiento operativo:** ante picos de tráfico, los componentes de análisis tienden a saturar la ruta crítica de la transacción.
- **Rigidez de las reglas estáticas:** los umbrales fijos no capturan desviaciones sutiles de comportamiento y generan altos índices de falsos positivos o falsos negativos.
- **Confusión de planos de detección:** se intenta resolver con una sola técnica problemas que son de naturaleza distinta —validaciones deterministas, anomalías puntuales y patrones de agregación—, lo que produce sistemas que detectan tarde lo que una regla simple habría frenado, o que nunca detectan lo que requiere análisis sobre conjuntos de eventos.
- **Detección sin respuesta:** los análisis asincrónicos generan alertas para un analista, pero no modifican el tratamiento de las operaciones siguientes del mismo actor, que continúan aprobándose mientras la alerta espera ser revisada.

El efecto combinado es una demora significativa entre la actividad anómala y la generación efectiva de una alerta, y entre la alerta y una mitigación real. En sectores donde los eventos se ejecutan en milisegundos, esta demora se traduce en pérdidas materializadas antes de cualquier mitigación manual.

## 2.3. Justificación de la solución

Se requiere una infraestructura orientada a eventos, desacoplada y desplegable de forma reproducible, capaz de ingerir ráfagas de transacciones concurrentes, analizarlas con baja latencia y aislar patrones anómalos. El aporte de ingeniería del proyecto consiste en asignar cada tipo de amenaza a la capa correcta: controles deterministas y de velocidad en la ruta sincrónica, anomalías puntuales en un scorer de streaming, y patrones de agregación en un analizador batch, manteniendo garantías operativas medibles en cada plano y un lazo de realimentación que convierte las detecciones asincrónicas en decisiones sincrónicas.

# 3. Estado del arte y posicionamiento

## 3.1. Soluciones comerciales

El mercado de detección de fraude transaccional cuenta con actores consolidados como Sift, Feedzai, Forter, Sardine, Unit21 y Kount, todos ellos basados mayoritariamente en arquitecturas cloud cerradas, con modelos propietarios entrenados sobre grandes volúmenes de datos agregados de sus clientes. Su principal foso defensivo no es arquitectónico sino de datos: el efecto de red de observar fraude a través de múltiples clientes simultáneamente. Estos sistemas ofrecen alta efectividad pero presentan limitaciones de costo, opacidad de modelos y dependencia de proveedor que dificultan su adopción en entidades de menor tamaño o con requisitos específicos de soberanía sobre los datos.

## 3.2. Soluciones de código abierto

En el espacio de código abierto existen componentes que abordan partes del problema: Apache Flink y Apache Kafka Streams permiten procesamiento de eventos en tiempo real; bibliotecas como scikit-learn, PyOD y River implementan modelos de detección de anomalías; frameworks de motores de reglas como Drools cubren la capa lógica. Sin embargo, no se identifica un sistema de referencia integrado, autocontenido y desplegable que combine de manera coherente todos estos elementos para el dominio específico del fraude de lógica de negocio.

## 3.3. Trabajos académicos relevantes

La detección de anomalías mediante modelos no supervisados es un área activa de investigación, con técnicas consolidadas como Isolation Forest (Liu, Ting y Zhou, 2008), Local Outlier Factor (Breunig et al., 2000), One-Class SVM y aproximaciones más recientes basadas en autoencoders y modelos de secuencia. La aplicación de estas técnicas al fraude transaccional cuenta con literatura extensa pero predominantemente enfocada en datasets offline. El trabajo propuesto se ubica en la intersección entre estas técnicas y la ingeniería de sistemas distribuidos para operación en tiempo real.

## 3.4. Posicionamiento del proyecto

El aporte central no consiste en proponer un algoritmo novedoso de detección sino en la integración arquitectónica: cómo construir un sistema EDA reproducible que combine análisis sincrónico de baja latencia, scoring de streaming y análisis batch, manteniendo garantías operativas medibles, una separación limpia entre controles deterministas y estadísticos, y un lazo cerrado entre la detección y la respuesta. Esta integración constituye el problema de ingeniería que el proyecto aborda como contribución principal. El Gateway adopta el modelo de API de decisión propio de los servicios comerciales de scoring de riesgo: el cliente consulta y recibe una decisión, en lugar de delegar su tráfico en un proxy. En términos de transferencia, el posicionamiento más realista no es competir con los actores globales en su terreno, sino atender nichos desatendidos: despliegue self-hosted con soberanía de datos, contexto regulatorio local y detección de abuso de lógica de negocio específico.

# 4. Objetivos

## 4.1. Objetivo general

Diseñar, desarrollar y validar un sistema distribuido y tolerante a fallos para la ingesta, análisis (sincrónico, de streaming y batch) y mitigación en tiempo real de anomalías operativas y fraudes de lógica en flujos transaccionales estructurados, mediante un lazo de realimentación entre las detecciones asincrónicas y las decisiones de la ruta crítica, evaluando su comportamiento en términos de latencia, throughput, consumer lag y precisión sobre un entorno de pruebas controlado.

## 4.2. Objetivos específicos

- Implementar un pipeline de ingesta asincrónica capaz de procesar eventos concurrentes con semántica de entrega documentada (al menos una vez, productor idempotente con acks=all) y resolución explícita de la tensión latencia / garantía de entrega (buffer acotado con descarte contabilizado).
- Desarrollar un Gateway concurrente en Go, concebido como API de decisión, con motor de reglas de velocidad sobre contadores atómicos en Redis, validaciones deterministas (idempotencia y firma de request, locks por recurso, validación de precio contra catálogo, cantidades inválidas), enriquecimiento de eventos (GeoIP, bloques de red, normalización de la huella de dispositivo informada por el cliente) y consulta del estado de riesgo del actor, con arranque seguro (readiness condicionado a la carga del estado de riesgo y de las políticas).
- Definir e implementar una política de falla por regla (matriz de comportamiento ante degradación) y el contrato HTTP del Gateway: decisión graduada (allow, review, step_up, block), clave de idempotencia y firma obligatorias, y evento de resultado informado por el cliente.
- Cerrar el lazo de mitigación mediante un estado de riesgo por actor, distribuido por un tópico compactado, con expiración (mediante tombstones) y override manual del analista, e incorporar como señal la resolución de los desafíos (step_up superado o fallido) informada en el evento de resultado.
- Integrar un scorer de streaming de análisis puntual no supervisado que evalúe los eventos por micro-lotes contra el perfil histórico del actor, mantenido de forma incremental en un perfil caliente protegido contra escrituras obsoletas durante los rebalanceos del consumer group, y que acompañe cada alerta con una explicación de las features que la motivaron.
- Desarrollar un analizador batch para patrones multi-evento y multi-entidad (triangulación, abuso de promociones, smurfing, comercio colusorio), apoyado en análisis de grafos de entidades compartidas, y para el monitoreo de salud del modelo, mediante jobs programados sobre la persistencia histórica.
- Versionar el esquema de eventos como contrato formal (JSON Schema como mínimo), con tipo de evento explícito, y particionar los tópicos por identificador de actor para garantizar el orden por entidad.
- Desacoplar la persistencia mediante un consumer group dedicado e idempotente, y gestionar políticas, catálogo, modelos y etiquetas del analista con mecanismos explícitos de distribución.
- Diseñar una interfaz reactiva de observabilidad, desacoplada del plano analítico mediante un servicio de notificaciones que puentee el bus de eventos hacia WebSockets, que incluya una cola de casos para las decisiones *review*.
- Garantizar la inmutabilidad y portabilidad del entorno mediante el empaquetado completo en contenedores con orquestación reproducible.
- Instrumentar el sistema con observabilidad interna (Prometheus/Grafana, con exportadores específicos para Kafka) y trazabilidad distribuida (OpenTelemetry, con el identificador de traza propagado en los headers de Kafka) desde la primera fase, como fuente de las métricas de evaluación del proyecto.
- Desarrollar un simulador de tráfico transaccional, ejecutable en un host separado, que inyecte de forma controlada patrones de comportamiento normal y anómalo etiquetado, y que permita registrar y reinyectar escenarios completos como logs de eventos.
- Definir y aplicar un protocolo de evaluación cuantitativa que incluya métricas de rendimiento (TPS, latencia, consumer lag por partición) y de precisión (matriz de confusión, F1), con escenarios congelados antes del ajuste de los detectores, repetibles de forma automática contra cada versión del sistema, y con pruebas de caos para la matriz de falla.
- Evaluar versiones alternativas del modelo en modo sombra (champion/challenger) sobre el mismo tráfico, sin que la versión candidata afecte las decisiones.
- Aplicar medidas de seguridad al propio sistema: autenticación del cliente hacia el Gateway, control de acceso por roles en el panel y tratamiento de datos personales conforme a la Ley 25.326.

# 5. Alcance y exclusiones

## 5.1. Inclusiones

El sistema procesará datos estructurados provenientes de eventos transaccionales simulados; evaluará reglas deterministas y de velocidad en la capa de transporte; identificará anomalías estadísticas puntuales a partir de perfiles de comportamiento histórico; y detectará patrones de agregación mediante análisis batch. El alcance incluye la implementación de todos los componentes descriptos en la sección 6, su integración como sistema único, el almacén de estado rápido, el estado de riesgo por actor y su realimentación al Gateway, el sink de persistencia, la distribución de políticas, el registro de modelos, el servicio de notificaciones, el simulador de tráfico, el panel de observabilidad, la observabilidad interna y la trazabilidad, las herramientas de repetición de escenarios y de pruebas de caos, y la documentación técnica de despliegue.

## 5.2. Exclusiones explícitas

- Procesamiento de datos crudos de tarjetas de crédito regulados bajo PCI-DSS.
- Operación como pasarela de procesamiento de pagos directa.
- Operación como proxy inverso o WAF del cliente: el Gateway es un servicio de decisión que solo observa los eventos que el cliente le informa. La defensa contra DDoS de capa 7 sobre la aplicación del cliente queda fuera; solo se cubre la autoprotección del propio Gateway.
- Requerimiento de hardware especializado de procesamiento gráfico (GPU) en el entorno cliente.
- Mitigación fuera de la decisión devuelta por el Gateway: el sistema decide (allow, review, step_up, block), pero la ejecución del desafío de autenticación o de la revisión corresponde al cliente simulado.
- Recolección de la huella de dispositivo: la genera la aplicación del cliente (o el simulador) y la informa en el evento; el Gateway solo la valida, normaliza y seudonimiza.
- Tolerancia a la caída del bus de eventos: el despliegue usa un único broker de Kafka, por lo que la pérdida del broker interrumpe el plano analítico (el Gateway sigue decidiendo con el estado materializado y descarta de forma contabilizada). La replicación del bus queda como trabajo futuro.
- Integración productiva del conector CDC (Debezium) con bases de datos de clientes reales: se contempla como componente opcional y, en su defecto, se emula desde el simulador.
- Detección de fraudes que requieren señales externas no recibidas por el sistema (datos de contracargos de las redes de tarjetas), que se declaran como trabajo futuro fundamentado.
- Capacidades de multi-tenancy de nivel productivo y certificaciones (SOC 2, ISO 27001), que pertenecen al eventual proceso posterior de transferencia tecnológica.

# 6. Marco tecnológico y arquitectura propuesta

## 6.1. Justificación de la elección arquitectónica

Se adopta una Arquitectura Orientada a Eventos (EDA) por su capacidad de desacoplar la ruta crítica de la transacción del análisis estadístico, permitiendo escalar ambos planos de forma independiente, absorber picos de tráfico mediante persistencia temporal en el bus de mensajería y aislar fallos sin comprometer el flujo completo.

Sobre la elección de Apache Kafka: para el volumen objetivo (referencia de 1.000 TPS en un único host) Kafka es, estrictamente, más de lo necesario, y existen alternativas más livianas. Se mantiene Kafka de forma deliberada por su valor pedagógico y por la riqueza de su ecosistema, con un único broker y múltiples particiones. Con un único broker no hay tolerancia a fallos del bus, y acks=all solo garantiza durabilidad en disco, no replicación; ambas limitaciones se declaran de forma explícita (sección 5.2). Desde la versión 4.0, KRaft es el único modo de operación (sin ZooKeeper), por lo que no constituye una decisión de diseño sino una consecuencia de la elección de versión. Esta decisión se documenta explícitamente como tal y no como omisión.

Sobre el rol del Gateway: se lo concibe como un **servicio de decisión** y no como un proxy inline. El cliente informa cada operación relevante (inicio de sesión, alta de cuenta, acción sobre el carrito, transacción, lectura) antes de ejecutarla y recibe una decisión; luego informa el **resultado** (concretada, cancelada o devuelta). Este modelo es el que utilizan los servicios comerciales de scoring de riesgo, hace que el fail-open sea un timeout del lado del cliente y evita que el sistema de detección se convierta en un punto único de falla del tráfico completo. La contrapartida es que el sistema solo observa lo que el cliente decide informar, lo cual se refleja en el alcance (secciones 5.2 y 7).

## 6.2. Componentes principales

| **Componente** | **Función** | **Tecnología** |
| --- | --- | --- |
| **Gateway concurrente (API de decisión)** | Recepción de eventos del cliente autenticados (API key o mTLS + firma HMAC), enriquecimiento (GeoIP, bloques de red, normalización y hash de la huella de dispositivo informada por el cliente), reglas de velocidad, validaciones deterministas, consulta del estado de riesgo y devolución de la decisión. Publica todos los eventos, aprobados y bloqueados, con decision y rules_fired. No acepta tráfico hasta haber materializado actor-risk y policies (readiness probe). Incluye autoprotección (load shedding). | Go (goroutines, canales) |
| **Almacén de estado rápido** | Dos instancias separadas. Instancia de contadores y perfiles: contadores de velocidad con ventana deslizante atómica (scripts Lua o GCRA) con TTL corto, y perfiles calientes incrementales por actor con TTL largo y número de secuencia por actualización; política de desalojo volatile-ttl y snapshot periódico de perfiles hacia la persistencia histórica para rehidratación. Instancia de control: claves de idempotencia, nonces y locks por recurso, con política noeviction. | Redis |
| **Bus de eventos** | Desacople total entre planos, persistencia temporal, tolerancia a picos. Tópicos events (particionado por actor_id), alerts, dlq, actor-risk (compactado) y policies (compactado). Productores con acks=all e idempotencia habilitada. Un único broker (sin replicación). | Apache Kafka (KRaft) |
| **Scorer de streaming** | Análisis puntual no supervisado sobre el perfil del actor, por micro-lotes (consume N registros y puntúa en una única llamada vectorizada). Es el único escritor de los perfiles calientes de los actores de sus particiones, con escrituras condicionadas por número de secuencia. Adjunta a cada alerta las features que la motivaron. Puede evaluar una versión candidata del modelo en modo sombra. Publica alertas y actualizaciones de riesgo; no persiste eventos. Consumer puro; FastAPI solo para administración y health checks. | Python + scikit-learn |
| **Analizador batch** | Patrones multi-evento y multi-entidad mediante jobs programados: triangulación, abuso de promociones, smurfing, comercio colusorio (con análisis de grafos de entidades compartidas) y bust-out (por features de trayectoria del actor). Monitoreo de salud del modelo (concept drift) y reentrenamiento con versionado. | Python + scikit-learn + NetworkX; planificación con APScheduler o cron en Compose |
| **Sink de persistencia** | Consumer group dedicado que escribe events, alerts y dlq en la base histórica de forma idempotente (ON CONFLICT (event_id) DO NOTHING). Su lag se mide de forma independiente del scorer. | Kafka Connect JDBC sink (preferido); alternativa: consumer en Go |
| **Persistencia histórica** | Series temporales, continuous aggregates para ventanas largas y arranque en frío, catálogo de productos, etiquetas del analista, snapshots de perfiles, auditoría y entrenamiento. Políticas de retención y compresión. | TimescaleDB / PostgreSQL |
| **Registro de modelos** | Almacenamiento de modelos entrenados con tabla de versiones; el scorer los recarga en caliente sin reiniciar y puede cargar una versión candidata en paralelo (modo sombra). | MinIO o volumen compartido + PostgreSQL |
| **Servicio de notificaciones** | Puente entre el tópico alerts y los WebSockets del panel. Su única responsabilidad es la entrega en tiempo real; desacopla la presentación del plano analítico. | Go |
| **Panel de observabilidad** | Interfaz reactiva para el analista y su backend: API REST de consulta, exploración y auditoría de eventos, cola de casos en *review*, visualización de la explicación de cada alerta, administración de políticas (publica en policies), feedback sobre alertas y override de riesgo (publica en actor-risk). Control de acceso por roles. | Next.js + TypeScript + Tailwind |
| **Observabilidad interna** | Recolección de métricas operativas (TPS, P50/P99, consumer lag por partición, eventos descartados) y trazas distribuidas (identificador de traza propagado en headers de Kafka) de todos los componentes. Para Kafka se usan JMX exporter y kafka-exporter. Fuente de las métricas de la tesis. | Prometheus + Grafana + OpenTelemetry |
| **Herramientas de evaluación** | Inyección de fallas de red y de dependencias (latencia, corte de Redis, broker lento) para validar la matriz de falla; repetición automática de escenarios congelados contra cada versión del sistema. | Toxiproxy; replay de logs de eventos |
| **Conector CDC (opcional)** | Captura de cambios de la base del cliente para reconciliar contra Kafka y detectar fraude interno. Componente opcional / trabajo futuro. | Debezium |
| **Orquestación** | Empaquetado reproducible, despliegue local y portabilidad del entorno. Integración continua con contenedores efímeros (testcontainers) y pruebas de contrato sobre el JSON Schema. | Docker + Docker Compose |
| **Simulador de tráfico** | Generación de eventos normales y anómalos etiquetados; emula al cliente (firma de requests, huella de dispositivo, eventos de resultado, cambios de catálogo). Registra los escenarios como logs reinyectables. Se ejecuta en host separado con relojes sincronizados para no contaminar las mediciones. | Python (desarrollo propio) |

Los tópicos del bus de eventos y su contenido se resumen a continuación.

| **Tópico** | **Contenido** | **Clave y retención** |
| --- | --- | --- |
| events | Todos los eventos evaluados por el Gateway, aprobados o bloqueados: inicios de sesión, altas, acciones de carrito, transacciones, lecturas y resultados. Campos mínimos: event_id, event_type, actor_id, decision, rules_fired, trace_id y atributos enriquecidos. | Clave actor_id. Retención por tiempo configurable. |
| alerts | Alertas del scorer y del analizador batch, con alert_id determinista: hash de (event_id, detector, versión del modelo). Incluyen la explicación (features y contribución de cada una). | Clave actor_id o entidad. Retención por tiempo. |
| dlq | Eventos malformados o venenosos, con la causa del descarte. | Retención extendida para análisis. |
| actor-risk | Estado de riesgo vigente por actor: nivel, motivo, fuente (scorer, batch o analista) y expiración. Al vencer, el emisor publica un tombstone (valor nulo) para que la compactación elimine la clave. | Clave actor_id. Compactado. |
| policies | Umbrales, listas y reglas vigentes administradas desde el panel. El Gateway las lee completas al arrancar y ante cada cambio. | Clave: identificador de política. Compactado. |

## 6.3. Flujo lógico de la información

Cada operación del cliente es informada al Gateway antes de ejecutarse, autenticada con API key y firma HMAC sobre el cuerpo, un timestamp y un nonce. El Gateway valida la firma y la ventana temporal; enriquece el evento (geolocalización por IP, bloque de red, normalización y hash de la huella de dispositivo recolectada por la aplicación del cliente); determina el actor_id (usuario autenticado, y en su defecto dispositivo y luego IP); consulta el estado de riesgo vigente del actor, materializado en memoria a partir del tópico actor-risk; evalúa las condiciones de velocidad con contadores atómicos en Redis; y aplica las validaciones deterministas: rechazo de nonces y claves de idempotencia repetidos mediante SET NX atómico (ataques de repetición), locks por recurso para cupones, saldos y puntos (condiciones de carrera), y verificación de precio contra el catálogo y de cantidades válidas. El resultado es una decisión graduada (allow, review, step_up o block).

Cada instancia del Gateway, antes de pasar a estado *ready*, consume desde el inicio los tópicos actor-risk y policies hasta alcanzar su último offset (end-offset al momento del arranque); de lo contrario arrancaría sin conocimiento del riesgo vigente y aprobaría a actores ya señalados. Mientras no esté lista, no recibe tráfico del balanceador.

Todo evento evaluado, aprobado o bloqueado, se publica en el tópico events con su decisión, las reglas disparadas y el identificador de traza, mediante un productor asincrónico idempotente y usando el identificador de actor como clave de partición para garantizar el orden por entidad. Publicar también los bloqueados es deliberado: los fallos de login alimentan la detección de toma de cuentas, los intentos rechazados caracterizan al atacante y el panel puede auditar los bloqueos. Ante un ataque que concentre volumen en un actor o una IP, los eventos bloqueados se muestrean y se acompañan de contadores agregados para evitar saturar una única partición. Posteriormente el cliente informa el resultado de la operación como un evento adicional.

El scorer de streaming consume del bus como parte de un consumer group, en micro-lotes: la evaluación de un modelo de scikit-learn fila a fila tiene un costo fijo por llamada que impide sostener el volumen objetivo con un único proceso, por lo que se consumen N registros, se calculan las features y se puntúa en una única llamada vectorizada, con un proceso por partición o grupo de particiones. El tamaño del lote y el tiempo máximo de espera constituyen un parámetro explícito del trade-off entre throughput y latencia evento → alerta, y se caracterizan experimentalmente. Como el particionado es por actor, cada instancia es la única escritora de los perfiles de sus actores y puede mantener el perfil caliente de forma incremental en Redis (media y varianza por el método de Welford o promedios exponenciales), complementado con los continuous aggregates de TimescaleDB para ventanas largas y arranque en frío —nunca mediante consultas de agregación ad hoc por evento—. Puntúa la transacción y, ante una desviación significativa, publica una alerta en alerts —con la explicación de las features que la motivaron— y, cuando corresponde, una actualización en actor-risk. Los eventos malformados se derivan a la cola de mensajes muertos (dlq) para no detener el consumo. El scorer no persiste eventos.

En paralelo, el analizador batch ejecuta periódicamente jobs sobre la persistencia histórica para detectar los patrones que requieren agregación sobre conjuntos de eventos —los casos de entidades que comparten atributos (dispositivo, dirección, tarjeta, comercio) se modelan como grafos y se analizan por componentes conexos—, y publica sus hallazgos también en alerts y actor-risk. Además monitorea la salud del modelo, reentrena y guarda cada nueva versión en el registro de modelos, desde donde el scorer la recarga en caliente. Los eventos etiquetados como fraude se excluyen del conjunto de entrenamiento para no contaminar el modelo.

Un sink de persistencia, con su propio consumer group, escribe events, alerts y dlq en TimescaleDB. De este modo, un reinicio o atraso del scorer no corta la historia ni deja ciegos al batch y al panel, y el lag de cada etapa es medible por separado. Las escrituras son idempotentes, ya que la entrega es al menos una vez.

El lazo se cierra con el estado de riesgo: cada instancia del Gateway consume actor-risk y lo materializa en memoria con su expiración, de modo que el siguiente evento del actor recibe una decisión acorde (por ejemplo step_up o block). El analista puede marcar cada alerta como verdadera o falsa; esas etiquetas se persisten, permiten calcular la precisión operativa por detector y pueden levantar o forzar un nivel de riesgo (override). Las decisiones *review* generan un caso en la cola del panel, y el evento transaction_outcome informado por el cliente cierra el ciclo de las decisiones graduadas: un step_up superado o una revisión aprobada por el analista reducen el nivel de riesgo del actor, y uno fallido o abandonado lo elevan. Esta señal se utiliza además como etiqueta débil para evaluar a los detectores. Las políticas administradas desde el panel se distribuyen por policies. El catálogo de precios pertenece al cliente: se carga mediante una API de administración del Gateway (en el simulador, mediante eventos de actualización de catálogo), se persiste en TimescaleDB y se mantiene en una caché versionada del Gateway que contempla promociones y descuentos vigentes para evitar falsos positivos.

El servicio de notificaciones consume el tópico alerts y lo puentea hacia el panel de observabilidad mediante WebSockets; la consulta histórica la atiende el backend del panel a través de su API REST. De este modo, un reinicio del scorer (por ejemplo, para recargar el modelo) no afecta las conexiones del analista. La observabilidad interna recolecta métricas y trazas de todos los componentes de forma continua.

## 6.4. Decisiones de diseño transversales

**Política de falla por regla.** No existe una política única: cada control se degrada según el daño que causa su omisión. En la ruta sincrónica no hay un «análisis» que pueda demorarse salvo el estado rápido, y un atacante puede intentar provocar esa degradación. Aplicar fail-open a todo abriría los controles de repetición y de doble gasto justo cuando el sistema está degradado. Si el propio Gateway no responde dentro del timeout, el cliente aprueba la operación (fail-open del lado del cliente); en la ruta del Gateway se aplica la siguiente matriz:

| **Control** | **Comportamiento ante falla** | **Fundamento** |
| --- | --- | --- |
| Reglas de velocidad (#1 a #4, #15) | Fail-open con timeout corto; se conserva el límite local grueso. | Perder análisis es preferible a perder ventas legítimas. |
| Estado de riesgo del actor | Se usa el último estado materializado en memoria, mientras no haya expirado. | La consulta es local; solo se degrada la actualización. |
| Idempotencia, nonce y locks por recurso (#14, #16) | Fail-closed (decisión review). El fallback local de ventana corta en memoria solo se habilita con enrutamiento sticky por actor_id (hash consistente en el balanceador); con más de una instancia sin sticky routing, la decisión degrada siempre a review. | Un atacante puede degradar el estado para habilitar replay y doble gasto; un fallback local por instancia no detecta el mismo nonce enviado a instancias distintas. |
| Precio y cantidades (#10) | Validación contra la última versión de catálogo en caché; sin caché válida, decisión review. | Control determinista local; no depende de Redis. |
| Autenticación y firma | Fail-closed siempre. | Sin identidad válida no hay decisión confiable. |
| Publicación a Kafka | Nunca bloquea la decisión; buffer acotado con descarte contabilizado. | Protege el P99; la pérdida queda medida. |

**Tensión latencia vs. garantía de entrega.** Como la publicación es asincrónica, el nivel de acks no incide en el P99 de la respuesta HTTP. La tensión real aparece cuando el buffer del productor se llena: bloquear rompe el P99 y descartar rompe la promesa de no pérdida. Se resuelve con un buffer acotado y descarte contabilizado, exponiendo events_dropped_total como métrica reportada. Se usa acks=all con idempotencia habilitada, lo cual con un único broker no tiene costo adicional, evita duplicados por reintentos del productor y garantiza durabilidad en disco, aunque no replicación. La garantía de entrega «al menos una vez» vale con broker disponible y sin desborde de buffer; como el «al menos una vez» también proviene del lado consumidor, los consumidores son idempotentes (ON CONFLICT (event_id) DO NOTHING para eventos y alert_id determinista para alertas). Si se requiriera la garantía fuerte, se contempla un patrón outbox local en disco como extensión, fuera del núcleo.

**Esquema de eventos como contrato.** En una EDA, el evento es la API. El esquema se versiona formalmente (JSON Schema como mínimo; Avro con Schema Registry como punto opcional) para que un cambio de campo no rompa simultáneamente Gateway, consumidores y simulador, y se verifica con pruebas de contrato en la integración continua. Incluye un event_type explícito (login_attempt, account_created, cart_action, transaction, read, transaction_outcome), dado que no todo evento es una transacción, un identificador de traza, y define el actor_id con un orden de respaldo (usuario, dispositivo, IP) para eventos no autenticados.

**Exclusividad de escritura del perfil y rebalanceos.** El particionado por actor hace que cada perfil tenga un único escritor *en régimen estable*, pero durante un rebalanceo del consumer group (caída, alta de una instancia, reinicio) puede coexistir brevemente un consumidor que perdió la partición con el que la recibió (consumidor zombie), y dos escrituras concurrentes sobre un mismo perfil corrompen la media y la varianza incrementales. Se mitiga en dos niveles: (1) se reduce la frecuencia de rebalanceos con membresía estática del grupo (group.instance.id) y asignación cooperativa; (2) toda actualización del perfil se aplica mediante un script Lua que compara el offset del último evento incorporado y rechaza las actualizaciones con offset menor o igual al registrado, de modo que una escritura obsoleta es descartada y la reentrega es idempotente. El comportamiento durante los rebalanceos se incluye en la batería de pruebas de caos.

**Durabilidad diferenciada de contadores y perfiles.** Los contadores de velocidad son descartables (su pérdida solo reinicia la ventana); los perfiles calientes no (su pérdida produce un arranque en frío masivo justo cuando un ataque presiona al sistema). Por eso la instancia de contadores y perfiles se configura con política de desalojo volatile-ttl, con TTL corto en los contadores y TTL largo en los perfiles, de modo que bajo presión de memoria se desalojan primero los contadores; además, se acota la cardinalidad de claves de contadores y se realiza un snapshot periódico de los perfiles hacia TimescaleDB para poder rehidratarlos tras una pérdida. El uso de memoria y las claves desalojadas son métricas expuestas.

**Estado de contadores y memoria del Gateway.** Redis es la única fuente de verdad de los contadores de velocidad, con operaciones atómicas (ventana deslizante en Lua o GCRA) y pipelining, lo que permite el escalado horizontal del Gateway sin que cada instancia cuente solo su parte. La memoria local se reserva como primera línea gruesa para proteger a Redis de un DDoS: mapas por IP/dispositivo/usuario con TTL agresivo, estructuras acotadas tipo LRU y sharding de locks para evitar que un único mutex global serialice las goroutines. Sin esta precaución, un ataque con IPs rotativas convertiría el componente anti-DDoS en el propio vector de DDoS por agotamiento de memoria. Las claves de idempotencia se alojan en una instancia separada con noeviction, de modo que un ataque no pueda provocar su desalojo y abrir un hueco de repetición.

**Estado de riesgo, expiración y respuesta graduada.** Los scorers, el batch y el analista actualizan el riesgo de un actor con un nivel, un motivo, una fuente y una expiración. Para limitar el costo de los falsos positivos, la respuesta por defecto ante una detección estadística es review o step_up; el bloqueo directo se reserva para detecciones de alta confianza o decisión del analista, y todo estado expira salvo renovación. La expiración se aplica en memoria en cada Gateway y, además, el emisor publica un tombstone al vencer, porque de otro modo el tópico compactado conservaría para siempre una clave por cada actor alguna vez señalado. El resultado informado por el cliente tras un step_up o una revisión (superado, fallido, abandonado) actualiza el nivel de riesgo y alimenta las métricas de precisión.

**Throughput del scoring de streaming.** El costo por llamada de los modelos de scikit-learn hace inviable puntuar evento por evento a 1.000 TPS. El scorer trabaja por micro-lotes (sección 6.3) y escala con un proceso por partición. El tamaño del lote, el tiempo máximo de espera y la cantidad de particiones son variables experimentales del trade-off entre throughput, latencia evento → alerta y lag, y su caracterización se reporta como resultado de la tesis.

**Modo sombra y comparación de modelos.** Dado que el registro de modelos versiona los artefactos, el scorer puede cargar una versión candidata y puntuar con ambas sobre el mismo tráfico: solo la versión vigente (champion) produce alertas y actualiza el riesgo; la candidata (challenger) registra sus puntajes en la persistencia para compararlos offline. Esto permite evaluar un reentrenamiento o un modelo alternativo sin exponer a los usuarios a sus falsos positivos.

**Explicabilidad mínima de las alertas.** Cada alerta incluye las features que más contribuyeron al puntaje (por ejemplo, el z-score por feature o la contribución de cada variable al aislamiento). Sin explicación, el analista no puede juzgar si una alerta es verdadera o falsa, y la precisión operativa por detector, calculada a partir de sus etiquetas, quedaría contaminada.

**Validación de precio contra catálogo: justificación.** Dado que el backend del cliente debería validar sus propios precios, esta verificación en el Gateway se justifica como defensa en profundidad: detecta manipulaciones que el cliente no controló, bugs de integración y divergencias entre el catálogo del cliente y el que se muestra a los usuarios. Su valor no es sustituir la validación del cliente sino ofrecer una segunda comprobación independiente y de costo mínimo, con el inconveniente de requerir un catálogo sincronizado, razón por la cual se la acompaña de una caché versionada y de la decisión review como falla segura.

**Consistencia de features.** El batch entrena con features calculados en SQL y el scorer los calcula en línea, lo que puede producir una divergencia entre entrenamiento y servicio (training-serving skew). Se define un módulo único de definición de features y un test automatizado que compara ambos cálculos.

**Trazabilidad distribuida.** Cada evento recibe en el Gateway un identificador de traza que se propaga en los headers de Kafka y se registra en events, alerts y actor-risk. De este modo la latencia evento → alerta y la latencia de mitigación (evento anómalo → primera decisión modificada) se miden sobre una misma traza y un mismo reloj, sin depender de la sincronización entre hosts para los intervalos internos del sistema.

**Seguridad del propio sistema y datos personales.** El cliente se autentica contra el Gateway con API key o mTLS y firma HMAC. El panel aplica control de acceso por roles (analista, administrador de políticas, auditor), porque quien modifica umbrales puede apagar la detección. La IP, la geolocalización y la huella de dispositivo son datos personales bajo la Ley 25.326: se aplica minimización, retención limitada, compresión y borrado por política en TimescaleDB y seudonimización del identificador de actor cuando es posible.

# 7. Taxonomía de fraudes y asignación a planos de detección

El catálogo de fraudes a cubrir no es homogéneo: mezcla tres clases de detección de naturaleza distinta. Pretender resolverlas todas con un único modelo puntual es un error de diseño. El alcance por defecto del proyecto es cubrir prácticamente todo el catálogo, asignando cada amenaza al plano que le corresponde. La siguiente tabla consolida esa asignación y marca, para cada caso, su estado de alcance: núcleo (compromiso firme), extendido (objetivo por defecto, degradable) o trabajo futuro (requiere señales externas no disponibles).

| **#** | **Fraude** | **Técnica de detección** | **Plano** | **Alcance** |
| --- | --- | --- | --- | --- |
| 1 | **Card Testing (prueba de tarjetas)** | Velocidad por IP/dispositivo, incluidos los intentos bloqueados | Gateway | Núcleo |
| 2 | **Credential Stuffing** | Velocidad de fallos de login (evento login_attempt) | Gateway | Núcleo |
| 3 | **Creación masiva de cuentas** | Volumen por bloque de red (CIDR) sobre el evento enriquecido | Gateway | Núcleo |
| 4 | **Inventory Hoarding / Scalping** | Velocidad sesión→compra (eventos cart_action) | Gateway | Extendido |
| 5 | **Autoprotección del Gateway (carga L7)** | Concurrencia, límite global y load shedding | Gateway | Núcleo |
| 6 | **Account Takeover (ATO)** | Desviación de perfil (Z-score, hora, dispositivo) y realimentación al Gateway vía actor-risk | Streaming | Núcleo |
| 7 | **Impossible Travel** | Velocidad geoespacial sobre geolocalización enriquecida | Streaming | Núcleo |
| 10 | **Manipulación de lógica (precio/cantidad)** | Validación determinista contra catálogo versionado | Gateway | Núcleo |
| 14 | **Replay Attacks** | Firma de request (HMAC + timestamp + nonce) y clave de idempotencia derivada del recurso de negocio | Gateway | Núcleo |
| 16 | **Race Conditions / Double Dipping** | Lock atómico por recurso (SET NX PX) + auditoría de timestamps | Gateway + Streaming | Núcleo |
| 15 | **API / Price Scraping** | Velocidad de lectura (requiere que el cliente informe las lecturas) | Gateway | Extendido |
| 18 | **Loyalty / Reward Points Laundering** | Tasa de canje de puntos (burn rate) | Streaming | Extendido |
| 8 | **Triangulation Fraud** | Cuenta→N direcciones; rotación de tarjetas (grafo cuenta–dirección–tarjeta, componentes conexos) | Batch | Extendido |
| 9 | **Promo / Referral Abuse** | Clustering por huella de dispositivo (grafo cuenta–dispositivo, componentes conexos) | Batch | Extendido |
| 11 | **Money Laundering / Smurfing** | Ratio entrada/salida + dwell time | Batch | Extendido |
| 19 | **Collusive Merchant (B2B)** | Approval rate y ratio de tarjetas nuevas por comercio; grafo comercio–tarjeta | Batch | Extendido |
| 12 | **Bust-out / Synthetic Identity** | Trayectoria de la cuenta: gasto de los últimos 7 días contra los últimos 90, aceleración de límites (features por actor) | Batch | Extendido |
| 13 | **Friendly / Chargeback Fraud** | Requiere datos externos de contracargos | — | Trabajo futuro |
| 17 | **Insider Threat** | Reconciliación CDC contra la BD del cliente | Batch + CDC | Trabajo futuro |

## 7.1. Lectura de la taxonomía

**Lo determinista vive en el Gateway, no en el modelo.** Manipulación de precio (#10) y replay (#14) no necesitan aprendizaje automático: son validaciones contra el catálogo y de unicidad que pertenecen a la ruta sincrónica. Detectar un precio adulterado segundos después de aprobar la compra sería un fracaso de diseño. Que el servicio de riesgo valide un precio que el backend del cliente debería validar por sí mismo se justifica como defensa en profundidad (sección 6.4).

**La idempotencia no equivale a protección contra repetición.** Una clave de idempotencia generada por el cliente protege contra reenvíos accidentales (reintentos, doble clic), no contra repetición maliciosa, porque un atacante simplemente envía una clave nueva. La protección contra replay (#14) se basa en la firma del request con timestamp y nonce, o en una clave derivada del recurso de negocio (por ejemplo, el identificador de orden o de sesión de compra). El doble gasto (#16) suele consistir en dos requests distintos que consumen el mismo recurso (cupón, saldo, puntos), por lo que se mitiga con un lock atómico por recurso y no por request; el análisis asincrónico de timestamps queda como auditoría complementaria.

**Los patrones de agregación exigen un plano batch propio.** Triangulación (#8), abuso de promociones (#9), smurfing (#11) y comercio colusorio (#19) no son anomalías de un evento sino de conjuntos de eventos y relaciones entre entidades. Por eso el plano analítico se divide: forzarlos dentro del scorer puntual produciría un monolito Python confuso e incapaz de mantener el SLA de streaming. Tres de ellos (#8, #9, #19) son, además, problemas de relaciones entre entidades que comparten atributos (dispositivo, dirección, tarjeta, comercio), por lo que se abordan modelándolos como grafos y analizando sus componentes conexos, técnica natural para este tipo de patrones.

**El bust-out (#12) es una trayectoria, no un drift.** Consiste en una cuenta que construye confianza y luego gasta de forma abrupta; se detecta con features por actor, no con detección de concept drift. El drift es un asunto distinto, de salud del modelo a nivel población, y se monitorea aparte en el analizador batch.

**Lo que el Gateway no ve, no lo detecta.** Al ser un servicio de decisión, solo observa lo que el cliente le informa. Por eso la carga L7 contra la aplicación del cliente (antes #5) se redefine como autoprotección del propio Gateway, y el scraping (#15) solo es detectable si el cliente informa las lecturas. Lo mismo vale para la huella de dispositivo, que debe ser recolectada por la aplicación del cliente.

**La detección asincrónica debe poder actuar.** Los fraudes asignados a streaming y batch (#6, #8, #9, #11, #12, #18, #19) mitigan a través del estado de riesgo del actor: la detección actualiza actor-risk y el Gateway modifica su decisión para las operaciones siguientes, con expiración y override del analista.

**Dos fraudes quedan honestamente fuera.** El fraude amistoso (#13) requiere datos de contracargos provenientes de las redes de tarjetas, una señal que el sistema no recibe por ningún flujo. La amenaza interna (#17) requiere comparar Kafka contra la base de facturación del cliente vía un conector CDC que solo se contempla como opción. Ambos se declaran como trabajo futuro fundamentado; en el simulador puede emularse parcialmente el caso #17 inyectando eventos sin gemelo de origen. Es preferible defender un conjunto bien medido que prometer cobertura total e indemostrable.

**Nota metodológica sobre la circularidad.** Quien diseña los ataques y los detectores corre el riesgo de evaluar lo que él mismo construyó. El caso más claro es el fingerprint (#9): si el simulador genera los device fingerprints que luego el modelo agrupa, la detección es parcialmente circular. Para mitigarlo, los escenarios de evaluación se congelan antes de ajustar los detectores y se almacenan como logs de eventos reinyectables, de modo que cada versión del sistema se evalúe contra exactamente los mismos escenarios (regresión reproducible); se procura que un subconjunto de los ataques sea diseñado por una persona distinta del autor y se complementa con datasets públicos. Estas limitaciones se discuten explícitamente en la tesis para no sobreestimar la transferibilidad del resultado.

# 8. Metodología de validación

## 8.1. Plan A: validación sobre simulador propio

La validación principal se realiza sobre un simulador de tráfico transaccional desarrollado como componente del proyecto, capaz de generar eventos sintéticos que reproduzcan patrones de comportamiento normal y patrones anómalos etiquetados. El simulador permite controlar densidad de tráfico, distribución temporal, tipos de ataques inyectados y proporción entre eventos legítimos y fraudulentos, y emula al cliente del Gateway (firma de requests, huella de dispositivo, eventos de resultado, cambios de catálogo). Para no contaminar las mediciones, el generador de carga se ejecuta en un host distinto del sistema bajo prueba (o, en su defecto, con límites de cgroup documentados) y los relojes de ambos hosts se sincronizan con chrony. Esta autonomía respecto a datos externos hace que la validación no dependa de factores fuera del alcance del proyecto. Los escenarios de evaluación se definen y congelan antes del ajuste de los detectores.

Cada escenario congelado se guarda como un log de eventos (con sus etiquetas y su cronología) y se dispone de un comando que lo reinyecta contra una versión dada del sistema. Con ello la evaluación es una regresión reproducible: cada cambio en un detector, en el modelo o en una política puede compararse contra exactamente los mismos escenarios, lo que además ayuda a acotar el sesgo de haber ajustado los detectores sobre los datos de evaluación.

## 8.2. Plan B (complementario): datasets públicos y validación externa

Como complemento, se contempla la utilización de datasets públicos de referencia en detección de fraude (por ejemplo, el dataset IEEE-CIS Fraud Detection u otros equivalentes) para validar la transferencia de las técnicas a datos con estructura no controlada por el autor. *Adicionalmente, podría contemplarse una validación con datos provistos por terceros, sujeta a disponibilidad y condiciones legales aplicables. Este plan se considera complementario, no es prerrequisito del proyecto.*

## 8.3. Métricas de rendimiento

- **Throughput sostenido (TPS):** objetivo de referencia 1.000 TPS en el Gateway, ajustable según hardware disponible.
- **Latencia P50 y P99 de la decisión:** objetivo P99 inferior a 10 ms de procesamiento en el Gateway (reglas en memoria más consultas a Redis) e inferior a 100 ms extremo a extremo incluyendo red y serialización. Para análisis de streaming, latencia end-to-end (evento → alerta) inferior a 5 segundos, medida a partir de las trazas distribuidas (ingreso al Gateway y publicación de la alerta, bajo un mismo identificador de traza).
- **Latencia de mitigación:** tiempo transcurrido entre el evento anómalo y la primera decisión modificada por el estado de riesgo en el Gateway, medida sobre la misma traza. Objetivo preliminar: no superior a 10 segundos.
- **Consumer lag de Kafka:** métrica central del SLA asincrónico; se monitorea por partición y de forma agregada, para los consumer groups del scorer y del sink de persistencia, mediante JMX exporter y kafka-exporter.
- **Trade-off del micro-lote:** curvas de throughput y de latencia evento → alerta del scorer al variar el tamaño del lote, el tiempo máximo de espera y la cantidad de procesos/particiones.
- **Estabilidad bajo carga:** ausencia de pérdida de eventos en escenarios de pico sostenido durante al menos 10 minutos, con events_dropped_total reportado y nulo dentro de la capacidad nominal.
- **Comportamiento ante fallas (pruebas de caos):** con inyección de fallas mediante Toxiproxy (latencia y corte de Redis, broker lento) y con reinicios y altas de instancias del scorer durante la carga, se verifica que cada control se degrade según la matriz de falla, que ningún perfil se corrompa durante los rebalanceos y que el Gateway no acepte tráfico antes de estar listo.

## 8.4. Métricas de precisión

- **Matriz de confusión completa:** verdaderos positivos, verdaderos negativos, falsos positivos y falsos negativos, calculada para el plano Gateway sobre los eventos persistidos, bloqueados incluidos.
- **Precisión, Recall y F1:** objetivo de F1 superior a 0,80 sobre los escenarios sintéticos del núcleo.
- **Análisis del balance Falsos Positivos / Falsos Negativos:** relevante para discutir el ajuste operativo del umbral del modelo, incluyendo el comportamiento del arranque en frío (actor sin historial) y el costo de los bloqueos erróneos del lazo de mitigación.
- **Precisión operativa por detector:** derivada de las etiquetas verdadero/falso positivo que asigna el analista a las alertas (apoyadas en la explicación adjunta a cada una) y de la señal de resultado de los desafíos step_up y de las revisiones.
- **Comparación champion/challenger:** para cada versión candidata evaluada en modo sombra, diferencia de F1, de tasa de alertas y de solapamiento de alertas respecto de la versión vigente sobre los mismos escenarios.

Los valores objetivo se presentan como referencia preliminar y serán refinados en la primera fase del proyecto, una vez caracterizado el entorno de pruebas y el hardware disponible. El aporte de mayor valor de la tesis no es alcanzar los umbrales sino caracterizar el comportamiento del sistema en sus límites: curvas de degradación de F1 al variar la proporción de fraude, trade-off entre P99 y consumer lag bajo carga creciente, trade-off de tamaño de lote del scorer, sensibilidad del cold start, efecto de una partición caliente cuando un atacante concentra volumen en una clave, comportamiento durante rebalanceos y comportamiento ante la caída de Redis según la matriz de falla.

# 9. Plan de trabajo y cronograma

El plazo estimado total es de 18 meses (agosto 2026 – febrero 2028), organizado en cuatro fases con hitos verificables. El cronograma es alcanzable pero ajustado; su viabilidad depende de sostener una dedicación semanal estable y de respetar la disciplina del núcleo mínimo. La integración y el debugging del sistema distribuido —no la escritura de cada componente— constituyen el principal consumidor de tiempo.

## 9.1. Fase I – Investigación e infraestructura base (meses 1 a 5)

- Definición del esquema de eventos versionado (JSON Schema) con tipos de evento, identificador de traza y regla de actor_id, con pruebas de contrato, y del esquema relacional y temporal en TimescaleDB.
- Levantamiento del clúster local en contenedores: Kafka en modo KRaft con tópicos events, alerts, dlq, actor-risk y policies, y las dos instancias de Redis (con las políticas de desalojo definidas).
- Desarrollo de la ingesta base en Go con publicación asincrónica idempotente (acks=all), buffer acotado con métrica de descartes y clave de partición por actor.
- Sink de persistencia con escrituras idempotentes hacia TimescaleDB (preferentemente con Kafka Connect JDBC).
- Definición del contrato HTTP del Gateway (decisión graduada, firma, evento de resultado) y de la matriz de política de falla.
- Instrumentación con Prometheus/Grafana desde el inicio, con JMX exporter y kafka-exporter para el lag por partición, y trazado con OpenTelemetry con propagación del identificador de traza en los headers de Kafka.
- Integración continua con contenedores efímeros (testcontainers).
- Primera versión del simulador de tráfico con generación de eventos normales, ejecutable en host separado y con relojes sincronizados, y con registro de escenarios como logs reinyectables.
- Refinamiento del estado del arte y definición precisa de métricas objetivo.

**Hito de cierre de fase:** ingesta funcional end-to-end con eventos sintéticos publicados, persistidos y consultables, y métricas operativas y trazas visibles en Grafana.

## 9.2. Fase II – Gateway, estado, scorer y lazo de mitigación (meses 6 a 10)

- Motor de reglas de velocidad sobre contadores atómicos en Redis, con una capa local gruesa (TTL/LRU y sharding de locks).
- Validaciones deterministas en el Gateway: firma y nonce (replay), locks por recurso (doble gasto), precio contra catálogo versionado y cantidades inválidas, con la política de falla por regla (incluida la decisión sobre enrutamiento sticky por actor).
- Enriquecimiento de eventos: GeoIP, bloques de red y normalización de la huella de dispositivo informada por el cliente.
- Arranque seguro del Gateway: readiness condicionado a la materialización completa de actor-risk y policies.
- Scorer de streaming en Python como consumer puro, por micro-lotes, con primer modelo no supervisado, perfiles calientes incrementales en Redis con escritura condicionada por offset (script Lua), membresía estática del grupo y continuous aggregates para ventanas largas; módulo único de features con test de equivalencia.
- Estado de riesgo: publicación en actor-risk por el scorer y consumo en el Gateway, con expiración y tombstones (lazo de mitigación cerrado en su versión mínima), e incorporación de la señal de resultado de step_up.
- Cola de mensajes muertos (dlq) para eventos venenosos.
- Extensión del simulador con patrones anómalos etiquetados de los planos Gateway y streaming, y definición y congelamiento de los escenarios de evaluación antes del ajuste de los detectores.
- Primera batería de pruebas de carga y precisión, midiendo TPS, P99, consumer lag por partición y latencia de mitigación.

**Hito de cierre de fase:** flujo completo desde ingesta hasta una decisión modificada por una alerta de streaming, funcionando con datos sintéticos, con el núcleo de fraudes detectado y medido.

## 9.3. Fase III – Analizador batch, observabilidad e integración (meses 11 a 14)

- Implementación del analizador batch con los detectores de agregación (triangulación, promo abuse, smurfing, comercio colusorio, bust-out), con análisis de grafos para los patrones de entidades compartidas, planificación de los jobs y monitoreo de salud del modelo.
- Registro de modelos con versionado y recarga en caliente en el scorer, y modo sombra para versiones candidatas.
- Explicación de features en cada alerta.
- Servicio de notificaciones que puentea el tópico alerts hacia WebSockets, desacoplando el panel del scorer.
- Desarrollo del panel de observabilidad en Next.js con comunicación reactiva, API REST de consulta, cola de casos *review*, visualización de la explicación de cada alerta, administración de políticas, feedback del analista, override de riesgo y control de acceso por roles.
- Hardening de despliegue, documentación técnica y mejoras de robustez, incluida la política de retención de datos personales.
- Iteración del modelo, ampliación de fraudes del alcance extendido y, opcionalmente, conector CDC emulado para el caso de amenaza interna.

**Hito de cierre de fase:** sistema integrado y demostrable, con los tres planos de detección operativos, lazo de mitigación completo, panel funcional y métricas reproducibles.

## 9.4. Fase IV – Validación final y escritura de tesis (meses 15 a 18)

- Ejecución de la batería final de pruebas controladas mediante la repetición de los escenarios congelados y caracterización del sistema en sus límites (partición caliente, caída de Redis, carga creciente, rebalanceos del consumer group), con la suite de caos basada en Toxiproxy.
- Comparación champion/challenger sobre los escenarios congelados.
- Análisis y discusión de resultados (curvas de degradación, trade-offs, tamaño de lote, cold start, precisión operativa por detector).
- Redacción de la memoria final del proyecto.
- Preparación de la defensa.

**Hito de cierre de fase:** documento final completo y defensa preparada.

# 10. Plan de gestión de riesgos

## 10.1. Núcleo mínimo defendible

Desde el inicio del proyecto se identifica un núcleo mínimo defendible que garantiza una tesis presentable incluso si componentes secundarios deben recortarse o degradarse. Este núcleo está conformado por:

- Gateway en Go como API de decisión, con motor de reglas de velocidad y validaciones deterministas funcionando, y con arranque seguro (readiness).
- Almacén de estado rápido (Redis) para contadores, idempotencia y perfiles calientes, con la separación de instancias y las políticas de desalojo definidas.
- Bus de eventos Apache Kafka operativo con productores idempotentes, consumidores y particionado por actor.
- Scorer de streaming con al menos un modelo no supervisado integrado, por micro-lotes y con escritura de perfiles protegida frente a rebalanceos.
- Lazo de mitigación en su versión mínima: estado de riesgo en actor-risk consumido por el Gateway, con expiración.
- Sink de persistencia y persistencia funcional en TimescaleDB, con perfiles pre-materializados.
- Simulador de tráfico con inyección de eventos normales y anómalos etiquetados.
- Observabilidad con métricas y trazas suficientes para medir las latencias de decisión y de mitigación.
- Cobertura del subconjunto de fraudes marcado como «núcleo» (nueve casos: #1, #2, #3, #5, #6, #7, #10, #14, #16).
- Métricas de rendimiento y precisión medidas y reportadas, incluido consumer lag por partición.

Todo lo que excede este núcleo —analizador batch completo (incluido el análisis de grafos), panel reactivo avanzado y cola de casos, servicio de notificaciones, registro de modelos con recarga en caliente y modo sombra, explicación de alertas, feedback y override del analista, fraudes del alcance extendido, repetición automática de escenarios y suite de caos más allá de la caída de Redis, conector CDC, comparación entre múltiples modelos— constituye el alcance ampliado, que es el objetivo por defecto del proyecto pero puede degradarse a una versión más simple sin comprometer la integridad académica del trabajo. La misma filosofía de degradación se aplica a los detectores de fraude, no solo a los componentes de infraestructura.

## 10.2. Riesgos identificados y mitigación

| **Riesgo** | **Probabilidad** | **Mitigación** |
| --- | --- | --- |
| **Cuello de botella en el feature engineering (consultas por evento a la base)** | Alta | Perfiles pre-materializados: perfil caliente incremental en Redis (único escritor por partición) y continuous aggregates para ventanas largas y arranque en frío. Nunca consultas de agregación ad hoc por evento. Es el punto donde el SLA de 5 segundos vive o muere. |
| **Throughput insuficiente del scorer en Python (costo por llamada de scikit-learn)** | Alta | Scoring por micro-lotes vectorizados, un proceso por partición y caracterización experimental del trade-off tamaño de lote vs latencia; degradación a Z-score multivariado vectorizado si el costo del modelo lo exige. |
| **Corrupción del perfil caliente por doble escritor durante rebalanceos del consumer group** | Media | Membresía estática y asignación cooperativa para reducir rebalanceos; escritura del perfil condicionada por offset mediante script Lua (se descartan actualizaciones obsoletas); prueba de caos específica con reinicios del scorer bajo carga. |
| **Tensión latencia P99 vs. garantía de no pérdida de eventos** | Media | Decisión documentada: productor asincrónico con acks=all e idempotencia, buffer acotado y descarte contabilizado (events_dropped_total); garantía «al menos una vez» con broker disponible y consumidores idempotentes. Patrón outbox como extensión opcional. |
| **Fuga de memoria en los mapas de contadores del Gateway bajo DDoS con IPs rotativas** | Media | Redis como única fuente de verdad de contadores; capa local gruesa con TTL agresivo, estructuras acotadas tipo LRU y sharding de locks para no serializar las goroutines. |
| **Desalojo de perfiles calientes en Redis por presión de memoria de los contadores** | Media | Política volatile-ttl con TTL corto en contadores y TTL largo en perfiles, acotamiento de la cardinalidad de claves, snapshot periódico de perfiles a TimescaleDB para rehidratación y métricas de memoria y claves desalojadas. |
| **Degradación o caída de Redis** | Media | Política de falla por regla (matriz de la sección 6.4), instancia separada noeviction para idempotencia y fallback local de ventana corta (con enrutamiento sticky por actor). Se prueba de forma explícita en la Fase IV con inyección de fallas. |
| **Fallback local de idempotencia ineficaz con varias instancias del Gateway** | Media | Enrutamiento sticky por actor_id en el balanceador o, en su defecto, degradación a decisión review mientras el estado de control no esté disponible; decisión registrada en la matriz de falla. |
| **Gateway que arranca sin conocer el estado de riesgo y aprueba actores ya señalados** | Media | Readiness probe condicionada al consumo completo de actor-risk y policies hasta el end-offset del arranque; tombstones para que los tópicos compactados no crezcan sin límite. |
| **Falsos positivos por mitigación automática (bloqueo de usuarios legítimos)** | Media | Respuesta graduada (review o step_up antes de block), expiración del estado de riesgo, override del analista, señal de resultado del step_up y medición del costo de los bloqueos erróneos. |
| **Procesamiento fuera de orden de eventos del mismo actor** | Media | Particionado de Kafka por actor_id, garantizando orden por entidad para ATO, viaje imposible y condiciones de carrera. |
| **Partición caliente por concentración de volumen en un actor o IP** | Media | Medición del lag por partición, muestreo de eventos bloqueados con contadores agregados y tratamiento como experimento de comportamiento en los límites. |
| **Pérdida del broker de Kafka (único broker, sin replicación)** | Baja | Se declara como exclusión y limitación; el Gateway sigue decidiendo con el estado materializado y descarta de forma contabilizada; la replicación queda como trabajo futuro. |
| **Divergencia entre entrenamiento y servicio (training-serving skew)** | Media | Módulo único de definición de features y test automatizado de equivalencia entre el cálculo batch y el cálculo en línea. |
| **Circularidad y sesgo en la evaluación** | Media | Congelamiento de escenarios antes del ajuste de detectores, repetición automática de escenarios como regresión, subconjunto de ataques diseñado por terceros cuando sea posible y validación complementaria con datasets públicos. |
| **Mediciones contaminadas por competencia de recursos o por relojes desincronizados** | Media | Simulador en host separado o con límites de cgroup documentados, relojes sincronizados con chrony, latencias internas medidas con trazas bajo un mismo reloj y consumer lag como métrica central. |
| **Atrasos en integración de componentes** | Media | Núcleo mínimo defendible, integración continua con contenedores efímeros, revisiones quincenales con dirección, hitos verificables por fase. |
| **Complejidad mayor a la prevista del modelo estadístico** | Media | Degradación: comenzar con Isolation Forest sobre features simples; si excede el tiempo, sustituir por detector de Z-score multivariado. |
| **Limitaciones de hardware para pruebas de carga** | Media | Ajustar valores objetivo de TPS al hardware disponible; reportar resultados relativos además de absolutos. |
| **Falta de datos reales para validación** | Alta | Plan A: simulador propio como herramienta central. Plan B: datasets públicos. La validación no depende de terceros. |
| **Sobrealcance: 16+ detectores a calidad de tesis** | Alta | Compromiso firme solo sobre el núcleo; el resto es objetivo por defecto degradable. Profundidad de medición por encima de amplitud de cobertura. |
| **Amplitud tecnológica (Go, Python, TypeScript, Kafka, Redis, TimescaleDB) que impida defender cada pieza en profundidad** | Media | Priorizar la documentación de decisiones y del razonamiento sobre cada componente del núcleo; degradar primero los componentes periféricos (panel, notificaciones). |
| **Tratamiento de datos personales (Ley 25.326)** | Baja | Datos sintéticos en la validación; el diseño contempla minimización, retención limitada, compresión y borrado por política. |
| **Pérdida de motivación o saturación** | Media | Hitos parciales celebrables, reuniones periódicas, posibilidad de presentación parcial en eventos académicos (CACIC, JAIIO). |

## 10.3. Compromiso de comunicación

Se propone una cadencia de reuniones quincenales con la dirección, complementadas con un informe escrito al cierre de cada fase. Ante cualquier desvío respecto al cronograma, la comunicación es inmediata y la decisión de degradar componentes se toma de manera consensuada antes de comprometer la integridad del proyecto.

# 11. Resultados esperados

Al cierre del proyecto se esperan los siguientes entregables:

- Código fuente del sistema completo, organizado en repositorios separados por componente y con documentación técnica.
- Contrato de decisión del Gateway y esquema de eventos versionado, junto con la matriz de política de falla.
- Simulador de tráfico transaccional con capacidad de inyección de patrones anómalos etiquetados de los tres planos de detección y de registro y repetición de escenarios.
- Entorno de despliegue reproducible mediante Docker Compose, incluyendo observabilidad interna y trazabilidad distribuida.
- Suite de pruebas de caos y de regresión sobre escenarios congelados, repetible contra cada versión del sistema.
- Memoria final del proyecto con desarrollo de marco teórico, arquitectura, metodología, resultados y discusión.
- Reporte cuantitativo de métricas de rendimiento (TPS, P99, consumer lag por partición, latencia de mitigación, trade-off de micro-lote) y de precisión (matriz de confusión, F1, comparación champion/challenger), con análisis de los límites del sistema.
- Eventualmente, presentación de resultados parciales en eventos académicos nacionales (CACIC, JAIIO, ASAI) como contribución asociada.

# 12. Solicitud a la dirección

Se solicita la dirección y codirección del presente proyecto, considerando la complementariedad temática entre arquitectura de sistemas distribuidos y ciberseguridad aplicada que el trabajo requiere. Quedo a disposición para coordinar una reunión donde discutir en detalle cualquier aspecto de la propuesta, ajustar el alcance si así lo consideran necesario y acordar la modalidad de seguimiento.

**[Nombre y Apellido]**

LU: [Número]

*[Correo de contacto]*

# Referencias preliminares

- Breunig, M. M., Kriegel, H. P., Ng, R. T., y Sander, J. (2000). LOF: Identifying Density-Based Local Outliers. ACM SIGMOD Record.
- Liu, F. T., Ting, K. M., y Zhou, Z. H. (2008). Isolation Forest. Eighth IEEE International Conference on Data Mining.
- Kreps, J., Narkhede, N., y Rao, J. (2011). Kafka: A Distributed Messaging System for Log Processing. Proceedings of the NetDB.
- Newman, S. (2015). Building Microservices: Designing Fine-Grained Systems. O’Reilly Media.
- Kleppmann, M. (2017). Designing Data-Intensive Applications. O’Reilly Media.
- Welford, B. P. (1962). Note on a Method for Calculating Corrected Sums of Squares and Products. Technometrics, 4(3).
- República Argentina (2000). Ley 25.326 de Protección de los Datos Personales.

*La revisión bibliográfica se profundizará y ampliará durante la Fase I del proyecto.*
