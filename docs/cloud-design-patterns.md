# Patrones de Diseño Cloud

## Introducción

Se implementaron tres patrones de diseño cloud en el proyecto, seleccionados por estar directamente presentes en la arquitectura y ser demostrables en tiempo de ejecución. Los patrones cubren dos dominios distintos: la comunicación entre microservicios (Publisher/Subscriber y Competing Consumers) y el aislamiento de ambientes (Bulkhead).

---

## Patrón 1 — Publisher/Subscriber

### Problema que resuelve

El servicio `vote` necesita comunicar cada voto al `worker` para que sea persistido en PostgreSQL. Una llamada HTTP directa entre ambos servicios crearía acoplamiento fuerte — si el `worker` está caído o lento, el `vote` fallaría también. Además, no habría forma de escalar el procesamiento de votos independientemente de la recepción.

### Solución aplicada

Se introduce Kafka como broker de mensajería entre `vote` y `worker`. El servicio `vote` actúa como **publisher**: cada vez que un usuario vota, publica un mensaje en el topic `votes` de Kafka. El servicio `worker` actúa como **subscriber**: está suscrito al topic y procesa cada mensaje de forma asíncrona, persistiendo el resultado en PostgreSQL.

```
Usuario
   │
   ▼
[vote — Java]  ──publica──▶  [Kafka topic: votes]  ──consume──▶  [worker — Go]  ──▶  [PostgreSQL]
                                                                        │
                                                            [result — Node.js] ◀── lee
```

### Dónde está implementado

El broker Kafka se despliega como parte del Helm chart en `infrastructure/`. La configuración del producer está en el servicio `vote` y la del consumer en el servicio `worker`. El despliegue se realiza automáticamente mediante el pipeline de infraestructura (`infra.yml`) en el `ops-repo`.

```yaml
# infrastructure/Chart.yaml — Kafka como dependencia
dependencies:
  - name: kafka
    repository: https://charts.bitnami.com/bitnami
```

### Beneficios obtenidos

El desacoplamiento entre `vote` y `worker` permite que ambos servicios escalen, fallen y se desplieguen de forma completamente independiente. Si el `worker` se reinicia, los mensajes permanecen en Kafka y son procesados cuando el servicio vuelve. Si hay un pico de votaciones, Kafka absorbe la carga sin que `vote` se vea afectado.

### Relación con otros patrones

Publisher/Subscriber es la base sobre la cual se implementa Competing Consumers — múltiples instancias del `worker` pueden suscribirse al mismo topic gracias a la naturaleza distribuida de Kafka.

---

## Patrón 2 — Competing Consumers

### Problema que resuelve

Con una sola réplica del `worker` consumiendo mensajes de Kafka, el procesamiento de votos es secuencial. Durante picos de carga — por ejemplo, al inicio de una votación masiva — la cola de mensajes crece más rápido de lo que el worker la vacía, introduciendo latencia creciente en la persistencia de votos.

### Solución aplicada

Se escala el `worker` a múltiples réplicas en Kubernetes. Todas las réplicas compiten por consumir mensajes del mismo topic de Kafka. Kafka distribuye los mensajes entre los consumers a través del mecanismo de **consumer groups**: cada réplica del `worker` pertenece al mismo consumer group, y Kafka asigna particiones distintas a cada una. Ningún mensaje es procesado dos veces.

```
                         ┌─▶  [worker pod 1]  ─▶  PostgreSQL
[Kafka topic: votes]  ───┼─▶  [worker pod 2]  ─▶  PostgreSQL
                         └─▶  [worker pod 3]  ─▶  PostgreSQL
                         
       Kafka asigna una partición por pod automáticamente
```

### Dónde está implementado

El cambio se realiza en el Helm chart del `worker` dentro del `app-repo`:

```yaml
# worker/chart/values.yaml
replicaCount: 3
```

Este valor indica a Kubernetes que debe mantener 3 réplicas del pod `worker` corriendo simultáneamente. Kafka reconoce las nuevas instancias del consumer group y rebalancea las particiones automáticamente.

### Requisito en Kafka

Para que 3 workers consuman en paralelo sin solapamiento, el topic `votes` debe tener al menos 3 particiones. Si el número de particiones es menor que el número de réplicas, algunos workers quedarán ociosos.

```yaml
# infrastructure/values.yaml — configuración de particiones
kafka:
  topics:
    - name: votes
      partitions: 3
      replicationFactor: 1
```

### Beneficios obtenidos

El throughput de procesamiento de votos escala horizontalmente de forma lineal — 3 workers procesan aproximadamente 3 veces más mensajes por segundo que 1. El escalado es transparente para `vote` y para `result`, que no necesitan conocer cuántos workers existen.

---

## Patrón 3 — Bulkhead

### Problema que resuelve

En un sistema con un único ambiente de despliegue, un error en una versión en prueba puede afectar directamente a usuarios en producción. Recursos como CPU, memoria y conexiones a base de datos son compartidos, por lo que un servicio con un bug que consume recursos excesivos puede degradar o tumbar servicios en producción.

### Solución aplicada

Se implementa el patrón Bulkhead mediante **namespaces de Kubernetes**: `staging` y `production`. Cada namespace es un compartimento estanco — los pods, servicios, ConfigMaps, Secrets y quotas de recursos son completamente independientes entre namespaces. Un fallo en `staging` no puede propagarse a `production`.

```
Azure Kubernetes Service (AKS)
├── namespace: staging
│   ├── vote pod(s)
│   ├── worker pod(s)       ← versión en validación
│   ├── result pod(s)
│   ├── kafka
│   └── postgresql
│
└── namespace: production
    ├── vote pod(s)
    ├── worker pod(s)       ← versión estable
    ├── result pod(s)
    ├── kafka
    └── postgresql
```

### Dónde está implementado

Los namespaces se crean automáticamente mediante la flag `--create-namespace` en los comandos Helm del pipeline. El namespace destino es determinado por el trigger recibido desde el `app-repo`:

```yaml
# ops-repo — pipeline infra.yml
- name: Deploy service chart
  run: |
    helm upgrade --install ${{ matrix.service }} ./${{ matrix.service }}/chart \
      --namespace ${{ inputs.environment }} \
      --create-namespace \
      --set image=${{ inputs.image_tag }}
```

Donde `inputs.environment` es `staging` o `production` según la rama de origen en el `app-repo` (`develop` o `main` respectivamente).

### Beneficios obtenidos

El equipo puede desplegar y validar libremente en `staging` sin ningún riesgo para `production`. Los recursos de cómputo del clúster AKS están lógicamente separados por namespace, impidiendo que un servicio con comportamiento anómalo en staging consuma recursos que production necesita. Adicionalmente, los Secrets de Kubernetes (credenciales de base de datos, tokens) son distintos por namespace, lo que garantiza aislamiento de datos entre ambientes.

### Relación con la estrategia de branching

El Bulkhead está directamente alineado con la estrategia GitFlow del `app-repo`: `develop` siempre despliega al bulkhead de `staging`, y `main` siempre despliega al bulkhead de `production`. La estrategia de branching y el patrón arquitectural se refuerzan mutuamente.

---

## Resumen comparativo

| Patrón | Categoría | Problema resuelto | Implementación |
|---|---|---|---|
| Publisher/Subscriber | Mensajería | Acoplamiento entre vote y worker | Kafka en `infrastructure/` |
| Competing Consumers | Mensajería | Throughput limitado por un solo worker | `replicaCount: 3` en `worker/chart/values.yaml` |
| Bulkhead | Resiliencia | Fallos de staging afectando production | Namespaces `staging` y `production` en AKS |

