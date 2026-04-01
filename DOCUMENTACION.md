# Documentación Integral del Proyecto: Arquitectura, Pipelines y Operaciones (Taller 1)

**Entregable Final de Taller DevOps y Microservicios**

Esta documentación ofrece una visión extensa y argumentada de todos los componentes implementados en el ecosistema, analizando teóricamente y fundamentando con el código fuente cada uno de los lineamientos dictados por el Taller.

Abarca desde la concepción del flujo de trabajo por medio de la agilidad, cruzando por la construcción de estrategias de versionamiento, la materialización de patrones de resiliencia Cloud, y culminando con el control automatizado del desliegue mediante Integración Continua (CI), Despliegue Continuo (CD) e Infraestructura como Código (IaC).

---

## 1. Definición Metodológica de Trabajo

El ciclo de vida del desarrollo está enmarcado bajo el marco de trabajo **Scrum**, operando mediante *sprints* de dos semanas.
Esto significa que todas las arquitecturas y flujos automatizados diseñados deben garantizar que:
1.  **Las Historias de Usuario (Features)** se prueben y desplieguen rápidamente en `staging` de forma constante.
2.  **El Incremento del Producto** al finalizar el sprint llegue integral y sin fricciones a `production`.

Para que esto sea una realidad sin comprometer la seguridad ni la estabilidad de la infraestructura subyacente, el proyecto aplica el principio de **Separación de Responsabilidades** (SoC - Separation of Concerns), segmentando el ecosistema en dos repositorios con enfoques estratégicos totalmente diferentes.

---

## 2. Estrategias de Ramificación (Branching Strategies)

Debido a que el código de aplicaciones (*Software*) y el código de servidores (*Infraestructura*) tienen ciclos de vida, dependencias y riesgos abismalmente distintos, es un error arquitectónico aplicar el mismo flujo de control de versiones a ambos horizontes. Se implementaron dos estrategias especializadas.

### 2.1. GitFlow para Desarrollo (`microservices-demo-pipeline` / app-repo)
> **Cubre el 2.5% - Estrategia para desarrolladores**

La lógica empresarial pertenece aquí. GitFlow es excelente porque proporciona trazabilidad y protección del esfuerzo del equipo de desarrollo, atándose perfectamente a Scrum.

*   **Fundamento:** Un desarrollador tarda días en culminar una "User Story". Necesita un entorno seguro (`feature/*`) que no dañe a los demás, y un entorno integrado (`develop`) donde probar con su equipo.
*   **Funcionamiento Técnico Implementado:**
    *   **`main`**: Refleja siempre el ambiente de producción (Código intocable de forma directa, cerrado el sprint).
    *   **`develop`**: Refleja siempre `staging`, el entorno validatorio actual del sprint.
    *   **`feature/*`**: Ramas exclusivas para el desarrollo de cada desarrollador.
    *   **Flujo CI Reflejado:** Todo PR de una `feature/xxx` dispara validaciones de *tests unitarios* (como `mvn test` o `go test`) para validar código puro. Sin embargo, la simple aprobación del PR no ejecuta rutinas productivas; es el evento de `push` (derivado del merge) hacia `develop` o `main` lo que detona la compilación de la imagen Docker y la publicación de cara al despliegue. Los pipelines de despliegue no corren nunca durante el PR, aislando así el ambiente de código no estabilizado.
    *   **Resolución de crisis (`hotfix/*`)**: Si producción se cae, las ramas de `hotfix/` nacen obligatoriamente desde `main`, se fixean y se devuelven al PR hacia `main` (desplegando de urgencia el hotfix) y paralelamente se envía a `develop` para que las futuras features no sobre-escriban el arreglo.

### 2.2. Trunk-Based Development para Operaciones (`microservices-demo-pipeline-ops` / ops-repo)
> **Cubre el 2.5% - Estrategia para operaciones**

A diferencia del software, la Infraestructura define un *"Estado Actual Deseado"*, no una acumulación de nuevas características a largo plazo. Aquí, un cambio significa "Queremos subir a Kafka v25" o "Queremos cambiar el tipo de Máquina Virtual de Azure".

*   **Fundamento:** Si un ingeniero DevOps cambia una versión de Helm o una configuración de Clúster en una rama que vive tres semanas, todos los push del resto de los desarrolladores fracasarán en ese periodo. Los cambios de DevOps impactan a la empresa entera de inmediato.
*   **Funcionamiento Técnico Implementado:**
    *   **Filosofía:** No hay `develop`, no hay `staging-branch`. Existe solo **el tronco (`main`)**.
    *   **Ramas efímeras (`fix/*`, `update/*`)**: Viven a lo mucho 1 o 2 días.
    *   **Comportamiento Agóstico del Entorno**: Aunque este repositorio es "agnóstico" al no depender de ramas diferenciadas por ambiente, no opera complétamente ciego. El entorno deseado (`staging` o `production`) se inyecta como parámetro en el *payload* desde el App-Repo. El workflow de Ops ejecuta lógica explícita que lee este parámetro y decide qué secretos (`KUBECONFIG`) usar y qué variables dictar en Helm. Adicionalmente, cuenta con un mecanismo de validación del origen (`EXPECTED_APP_REPO`) que rechaza el despliegue si los gatillos provienen de repositorios u orígenes desconocidos, blindando la seguridad.

---

## 3. Topología de los Servicios

El sistema no es un monolito; consta de cinco actores técnicos que garantizan la resiliencia del sistema final.
1.  **Vote-App (Java/Spring Boot)**: Front-End emisor de datos asíncronos.
2.  **Worker-App (Go)**: Consumidor ultrarrápido con alto performance computacional que se encarga de guardar permanentemente lo que viene volando, dándole la validación y transformación final.
3.  **Result-App (Node.js)**: Front-End de consumo en vivo para visualizar encuestas activas.
4.  **Broker (Apache Kafka en KRaft mode)**: El puente intermedio para que los productores no ahoguen a los consumidores de back-end.
5.  **Motor Transaccional (PostgreSQL)**: Única fuente de la verdad para el persistencia en disco de los votos.

---

## 4. Patrones de Diseño Cloud Implementados (15%)

El aspecto más complejo del proyecto fue solventar las debilidades de un monolito frágil frente a internet y picos de tráfico. Se instrumentaron 3 Patrones de Cloud Architecture clave.

### 4.1. Patrón *Publisher/Subscriber* (Mensajería Basada en Eventos)
**Problema original:** El servicio `vote` y el servicio `worker` poseían acoplamiento fuerte. Si la base de datos ralentizaba al `worker`, el microservicio Java original (vote) haría *Timeout HTTP*, frustrando al votante y provocando caída en cascada (Cascading Failure).
**Solución en código:**
*   **Implementación:** Se introduce Apache Kafka desplegado como Chart de Helm dentro de la infraestructura base del clúster (Ops-Repo -> `infrastructure/k8s/templates/kafka.yaml`).
*   **El código:** El `vote` (Java) inicializa `KafkaTemplate` y envía un registro llaves-valor (Publish) al Topic `votes`, sin importarle quién lo lea y sin esperar respuesta de inserción en Base de Datos. El `worker` (Go) lee (Subscribe) este canal de manera constante. Si el `worker` colapsa, los mensajes no se pierden ya que Kafka los retiene de acuerdo a las *políticas de retención* configuradas en el broker (no indefinidamente), permitiendo que un consumer recupere el ritmo tras ser reiniciado.

### 4.2. Patrón *Competing Consumers* (Consumidores en Competencia / Escalabilidad)
**Problema original:** Al lanzar una encuesta a cientos de personas simultáneas, el *Pub/Sub* clásico solo lograba que Kafka retuviera los votos sanos (evitando timeout), pero un único hilo de Go (`worker`) era demasiado lento, generando horas de latencia para ver reflejado un resultado en Node.js.
**Solución en código:**
*   Se escaló radicalmente gracias a la paralelización. No bastó con solo triplicar el pod de Go. Se atacaron tres frentes en el taller:
    1.  **En Infraestructura:** Se creó un script de Kubernetes nativo (`kafka-topic-init.yaml` - Job de Helm Hook) que inyecta en el servidor de Kafka la orden de crear el topic `votes` exigiendo **3 Particiones**.
    2.  **En Kubernetes (Helm):** Se alteró el repositorio DEV y OPS (`worker/chart/values.yaml`) forzando que se instancien **`replicaCount: 3`**. Kubernetes levanta 3 contendeores de Go en pods paralelos.
    3.  **En Back-End (Go)**: Modificamos `worker/main.go`. Re-escribimos el consumo para abandonar la lectura directa (donde los 3 pods hubieran procesado el mismo voto y creado Data Corruption), y la migramos a una interfaz de `sarama.ConsumerGroup` (Bajo el string `"worker-group"`). De esta manera, Kafka balancea dinámicamente cada partición a un worker. Es clave comprender que el grado máximo de paralelismo concurrente dictado por el patrón tiene un techo rígido atado al número de particiones del broker; además, la malla de consumidores está sujeta a eventos de *rebalance* que pueden congelar fracciones de segundo la lectura al perderse un nodo de la red.

### 4.3. Patrón *Bulkhead* (Compartimentación y Límites de Fallo)
**Problema original:** Teniendo un solo clúster, si el equipo despliega "develop" y el código entra en lazo infinito o acapara Memoria/Hilos, derribaría el clúster entero, tumbando el servicio a los clientes en "main/producción".
**Solución en código:**
*   Se divide el Clúster de manera lógica similar a como los barcos separan su casco en secciones herméticas ("Bulkheads"). Si el cuarto de máquinas se inunda en staging, no afecta a Producción.
*   **Implementación**: Azure Kubernetes Service ofrece los **Namespaces**. Nuestro pipeline final del Ops-Repo fuerza el aislamiento corriendo en `ops.yml`:
    ```bash
    helm upgrade --install [...] --namespace "$ENVIRONMENT" --create-namespace
    ```
    Cabe recalcar que los Namespaces dictaminan una segmentación lógica pero **no representan un aislamiento físico absoluto** por sí mismos. Si no se instrumentan cuotas de recursos integrales (Resource Quotas) o Network Policies estables, un namespace sobre-saturado podría estrangular computacionalmente al clúster entero, violando la integridad de producción pese a operar separados, especialmente si ambos dependen y apuntan al idéntico contexto o conjunto de credenciales cruzadas en KUBECONFIG.

---

## 5. Diseño de CI y CD Integrado (Pipelines Automáticos) (20%)

El flujo "Zero Touch" (nada debe hacerse manualmente, solo *mergear*) requirió Github Actions de altísimo nivel, orquestando una comunicación de Inter-repositorios por Webhooks.

### 5.1. El Pipeline de Integración (App-Repo -> `app.yml`)
Este archivo de YAML (de casi 300 líneas) gobierna la salud del software:
1.  **Detección de Archivos y Matrices Dinámicas (Paths-filter)**: Usamos lógica condicional para evitar gastar cuotas. Si un desarrollador altera únicamente documentación, el Job de `test` queda en estado `skipped`. Esto no denota rotura de pipeline, sino que el action decide dinámicamente achicar el tamaño de la matriz de ejecución, o dejarla inclusive vacía; en lugar de tener "siempre matrices súper eficientes", ejecutamos estrictamente las piezas con *commits* incidentes.
2.  **Containerización y Versionado**: Se generan imágenes etiquetadas primariamente sobre `SHA` de commit (`ghcr.io/...vote:sha-h5jd8...`) para inmutabilidad. Es importante aclarar que esto no inhibe la generación e imposición de tags adyacentes semánticos como `:develop` o `:latest` en el registry publicados por la rama destino, no obstante, el despliegue efectivo en clúster debe hacerse preferentemente consumiendo el *tag inmutable SHA*.
3.  **El Puente Intersistémico (`repository_dispatch`)**: Al completar la contrucción, se transacciona un llamado a la API directa de GitHub usando un token de personal de acceso (`OPS_REPO_TOKEN`). Este contrato expone explícitamente el `event_type: "app-image-ready"`, acompañado de un `client_payload` (JSON) que estructura rígidamente al repositorio Ops qué originó el cambio, en qué ambiente y bajo qué SHA inmutable.

### 5.2. El Pipeline de Despliegue (Ops-Repo -> `ops.yml` y `infra.yml`)
*   **`ops.yml` (El oyente de Aplicaciones y Orquestador)**: Se dispara ante el evento condicionado de un `repository_dispatch`. Su lógica filtra e inspecciona el metadato del origen (`EXPECTED_APP_REPO`) antes de dar paso. Una vez convalidado, decide operativamente contra qué clúster/kubeconfig dialogar en concordancia al paramétro `environment`. Al resolver, despacha `Helm upgrade` buscando la imagen sembrada con `sha-XXX`.
*   **`infra.yml` (El constructor base)**: Es un canal propio, que si el ingeniero de la nube modifica los manifests de Postgre o Kafka, corre de forma estricta contra dichos ambientes inyectando la base fundacional.

---

## 6. Infraestructura como Software (IaC) (20%)

Con el objetivo de acercarse al marco de desatención operativa, la infraestructura prioriza el aprovisionamiento automatizado, aceptando la salvedad de que los ecosistemas perimetrales de identidad del cloud (ej. Secret Management y credenciales primarias) exigen intervenciones controladas manuales externas.
1.  **Terraform:** Modela y provisiona recursos Físicos/Virtuales absolutos. Contenemos configuraciones que exigen que Microsoft Cloud otorgue al Team la cuenta `rg-taller-1` montada en `eastus2`, proveyendo el recurso `azurerm_kubernetes_cluster`. Almacena en `terraform.tfstate` la trazabilidad de esta propiedad.
2.  **Helm:** Modela los contenedores frente al clúster subyacente de Kubernetes. Hemos transformado tres servicios dispares en plantillas lógicas para la fácil alteración de parámetros (Puertos, Imágenes, Nombres), y se absorben dependencias pesadas empresariales externas (Como los Charts de Bitnami de Zookeeper/Kafka) dentro un simple control por archivo `values.yaml`.

---

## 7. Instrucciones para la Demostración en Vivo (Evaluación Punto 8 - 15%)

Para el evaluador, así se demuestra que toda esta red de tecnologías complejas están unidas sin fallas. Es vital demostrar la automatización.

1.  **Entorno Inicial**: Confirme en su terminal que Kubernetes le permite consultar `kubectl get pods -n staging`. Confirme que Terraform fue aplicado.
2.  **Generación del "Feature"**: Sitúese en el terminal dentro de la carpeta del app-repo (`microservices-demo-pipeline`) para hacer mímica de una nueva historia de usuario.
    ```bash
    git checkout develop
    git checkout -b feature/pruebaviva-1
    ```
3.  **Mutación del Aplicativo**:
    Ingrese al código Java o Go. Por ejemplo, abra el `vote/src/main/resources/templates/index.html` (o `application.properties`) o simplemente haga un cambio menor que pueda notarse. O altere el archivo principal del Go worker agregando un log en consola nuevo (`fmt.Printf("Prueba Taller DevOps...\n")`).
4.  **Confirmación y Disparo de Pruebas**:
    ```bash
    git add .
    git commit -m "feat(demo): demostracion de la integracion continua en vivo"
    git push origin feature/pruebaviva-1
    ```
5.  **Aprobación y Merge a Clúster Real**:
    *   Haga el PR de `feature/pruebaviva-1` hacia `develop`.
    *   Revise los GitHub Action *Checks* que garantizan el pase de los tests automáticos en el PR. (Contar con Reglas de Protección Activas de rama obligaría al bloqueo de integración si los tests se encuentran decaídos).
    *   Apruebe y concrete el Merge a `develop`.
6.  **Observar la Cadena DevOps (The Magic)**:
    *   Entre a GitHub (Pestaña Actions).
    *   Vea cómo App-Repo arranca el `changes -> build-and-push -> trigger-ops-deploy`.
    *   Vea cómo Ops-Repo se despierta por arte de magia y su Action comienza el `ops.yml`. Se verá cómo se inyecta en "staging".
    *   Vaya a la terminal y confirme:
        ```bash
        kubectl get pods -n staging
        ```
        Verifique cómo los pods antiguos del trabajador terminan su ciclo gradualmente e inician 3 pods nuevos de Go.
    *   Realice pruebas desde el Frontend de Votos contra esa URL de staging y verifique que llegan muy fluidos al PostgreSQL gracias al Bulkhead apartado y ConsumerGroups de Kafka.


