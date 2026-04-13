# Microservices Demo Pipeline

Este repositorio contiene los servicios de la aplicacion de votacion y el pipeline de CI del app-repo.
El despliegue se ejecuta en el ops-repo mediante eventos `repository_dispatch` y tags inmutables.

Flujo base:

- app-repo construye y publica imagenes.
- app-repo dispara ops-repo con el tag generado.
- ops-repo despliega con Helm usando valores en runtime (sin mutar archivos Git).

## Arquitectura

![Architecture diagram](architecture.png)

- Vote service (Java, Spring Boot): [vote](vote)
- Result service (Node.js): [result](result)
- Worker service (Go): [worker](worker)
- Mensajeria: Kafka
- Persistencia: PostgreSQL

## CI/CD en 30 segundos

- `pull_request` desde `feature/*`: deteccion de cambios + tests por servicio.
- `push` a `develop`: build/push de imagenes + trigger a ops-repo para `staging`.
- `push` a `main`: build/push de imagenes + trigger a ops-repo para `production`.
- Tag de despliegue recomendado: `sha-<commit>` (inmutable).

Workflow principal de este repo: [.github/workflows/app.yml](.github/workflows/app.yml)

## Documentacion general del taller

### 1. Definicion metodologica de trabajo

El ciclo de vida del desarrollo esta enmarcado bajo Scrum, operando mediante sprints de dos semanas. Todas las arquitecturas y flujos automatizados estan orientados a garantizar que:

1. Las historias de usuario se validen y desplieguen rapido en staging.
2. El incremento del producto llegue estable y sin friccion a production al cierre del sprint.

El proyecto aplica separacion de responsabilidades (SoC), segmentando el ecosistema en dos repositorios con enfoques distintos: app-repo para software y ops-repo para operaciones.

### 2. Estrategias de ramificacion

Debido a que software e infraestructura tienen ciclos de vida y riesgos diferentes, se aplican estrategias especializadas.

#### 2.1 GitFlow para desarrollo (app-repo)

- `main`: estado de produccion.
- `develop`: estado de staging.
- `feature/*`: ramas de desarrollo por historia.
- `hotfix/*`: correcciones urgentes nacidas desde `main` y reintegradas a `main` y `develop`.

Flujo CI asociado:

- PR desde `feature/*`: validacion de tests (sin despliegue).
- Push por merge a `develop` o `main`: build de imagen + publicacion + trigger a despliegue en ops-repo.

#### 2.2 Trunk-Based para operaciones (ops-repo)

- Rama principal unica: `main`.
- Ramas efimeras de corta vida: `fix/*`, `update/*`.
- Entorno destino (`staging` o `production`) se inyecta por payload desde app-repo.
- Validacion de origen (`EXPECTED_APP_REPO`) para reforzar seguridad del disparo.

Detalle operativo: [docs/branching-strategy-dev.md](docs/branching-strategy-dev.md)

### 3. Topologia de servicios

El sistema consta de cinco actores tecnicos:

1. Vote-App (Java/Spring Boot): emisor de datos asincronos.
2. Worker-App (Go): consumidor y persistencia de votos.
3. Result-App (Node.js): visualizacion en vivo.
4. Kafka: puente de mensajeria entre productores y consumidores.
5. PostgreSQL: fuente de verdad transaccional.

### 4. Patrones de diseno cloud implementados

#### 4.1 Competing Consumers

Problema original: un solo worker no da throughput suficiente en picos de carga.

Solucion aplicada:

1. Topic `votes` con 3 particiones para paralelismo real.
2. `worker/chart/values.yaml` con `replicaCount: 3`.
3. Consumo en Go migrado a `sarama.ConsumerGroup` (`worker-group`) para reparto seguro de particiones sin duplicidad de procesamiento.

#### 4.2 Bulkhead

Problema original: un fallo de staging podia degradar produccion en un cluster compartido.

Solucion aplicada:

- Aislamiento logico por namespaces (`staging`, `production`) en AKS.
- Despliegue con Helm usando `--namespace` y `--create-namespace`.

Riesgo conocido: el namespace no es aislamiento fisico absoluto; sin quotas y politicas de red, un entorno puede tensionar recursos compartidos del cluster.

#### 4.3 Retry

Problema original: en un sistema distribuido, fallos transitorios de red (Kafka/DB no disponible temporalmente) pueden causar errores permanentes o perdida de mensajes.

Solucion aplicada:

- `vote` implementa reintentos en producer Kafka (retries + backoff fijo) para tolerar fallos breves al publicar votos.
- `worker` implementa reconexion con backoff exponencial al iniciar conexion a Kafka y PostgreSQL.
- Se reduce riesgo de perdida de votos y se evita saturar dependencias durante reinicios.

Detalle arquitectonico y resiliencia: [docs/cloud-design-patterns.md](docs/cloud-design-patterns.md)

### 5. Diseno CI/CD integrado

#### 5.1 Pipeline de integracion (app-repo -> app.yml)

1. Deteccion de cambios y matriz dinamica para ejecutar solo jobs relevantes.
2. Build y versionado con tag inmutable `sha-<commit>`.
3. Disparo inter-repositorio via `repository_dispatch` con `OPS_REPO_TOKEN` y payload estructurado (`service`, `image`, `tag`, `environment`, metadatos de origen).

#### 5.2 Pipeline de despliegue (ops-repo -> ops.yml / infra.yml)

- `ops.yml` escucha `repository_dispatch`, valida origen y despliega segun entorno.
- `infra.yml` despliega y mantiene componentes base de infraestructura (Kafka y PostgreSQL).

### 6. Infraestructura como codigo (IaC)

1. Terraform modela y aprovisiona recursos de cloud/cluster.
2. Helm modela y despliega servicios y componentes sobre Kubernetes.

### 7. Instrucciones de demostracion en vivo

Secuencia recomendada para evaluacion:

1. Confirmar entorno: `kubectl get pods -n staging`.
2. Crear rama `feature/*` desde `develop`.
3. Aplicar cambio pequeno en vote/worker/result.
4. Push de rama y apertura de PR a `develop`.
5. Validar checks y hacer merge.
6. Verificar cadena en Actions (`changes -> build-and-push -> trigger-ops-deploy`) y validar despliegue en staging.
7. Promover con PR `develop -> main` y validar production.

Guia corta paso a paso: [docs/demo-runbook.md](docs/demo-runbook.md)

## Configuracion requerida en app-repo

Secrets de GitHub Actions:

- `OPS_REPO_TOKEN`: token con permisos para enviar `repository_dispatch` al ops-repo.

Variables de GitHub Actions:

- `OPS_REPO`: repositorio destino en formato `owner/repo`.

Notas:

- No se requiere `KUBECONFIG` en este repo.
- Credenciales de cluster y ejecucion de Helm/kubectl pertenecen al ops-repo.
- El ops-repo está en: https://github.com/Juanpapb0401/microservices-demo-pipeline-ops

## Inventario de servicios

- `vote`: Java, Maven, Docker, Helm chart
- `result`: Node.js, npm, Docker, Helm chart
- `worker`: Go, go modules, Docker, Helm chart
