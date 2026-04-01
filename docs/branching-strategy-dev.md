# Branching Strategy

## Metodología ágil adoptada: Scrum

El equipo trabaja bajo **Scrum**, organizando el trabajo en sprints de dos semanas. Cada sprint tiene un objetivo claro y un conjunto de user stories priorizadas en el backlog. Las ceremonias incluyen sprint planning, daily standup, sprint review y retrospectiva.

La estrategia de branching está alineada con el ciclo de Scrum: las ramas `feature/*` corresponden a user stories del sprint activo, `develop` representa el incremento integrado del sprint, y `main` refleja el producto en producción al final de cada sprint.

El proyecto está separado en dos repositorios con responsabilidades distintas. Cada uno adopta una estrategia de branching diferente y tiene un rol exclusivo dentro del ciclo CI/CD:

| Repositorio | Estrategia | Rol en CI/CD |
|---|---|---|
| `app-repo` | **GitFlow** | CI — build, test y publicación de imágenes |
| `ops-repo` | **Trunk-Based Development** | CD — despliegue de infraestructura y microservicios |

---

## 1. Estrategia de branching para desarrollo — GitFlow

### Repositorio: `app-repo`

Contiene el código fuente de los microservicios (`vote`, `result`, `worker`), sus Dockerfiles y sus Helm charts de despliegue. Su responsabilidad termina en la publicación de la imagen en ACR — **no despliega directamente**.

### Ramas permanentes

| Rama | Propósito | Protegida | Trigger hacia ops-repo |
|---|---|---|---|
| `main` | Estado de producción | Sí — requiere PR aprobado | Dispara deploy a `production` |
| `develop` | Integración del sprint actual | Sí — requiere PR aprobado | Dispara deploy a `staging` |

### Ramas de trabajo (vida corta)

| Patrón | Se crea desde | Merge hacia | Cuándo usarla |
|---|---|---|---|
| `feature/<descripción>` | `develop` | `develop` vía PR | User story del sprint activo |
| `hotfix/<descripción>` | `main` | `main` + `develop` vía PR | Bug crítico en producción |

### Flujo de una user story (feature)

```
1. Developer toma una user story del sprint backlog

2. Crea rama desde develop:
   git checkout develop
   git checkout -b feature/vote-add-third-option

3. Trabaja en commits locales

4. Abre PR hacia develop
   → GitHub Actions ejecuta tests automáticos por servicio:
     mvn test  (vote)
     npm test  (result)
     go test   (worker)
   → Code review por otro miembro del equipo
   → Merge aprobado

5. Push a develop
   → GitHub Actions: build de imagen Docker por servicio modificado
   → Push a ACR con tag inmutable sha-<commit>
   → Trigger al ops-repo con tag generado + environment=staging

6. Al cierre del sprint, PR de develop → main
   → Code review
   → Merge aprobado

7. Push a main
   → GitHub Actions: build de imagen Docker por servicio modificado
   → Push a ACR con tag inmutable sha-<commit>
   → Trigger al ops-repo con tag generado + environment=production
```

### Flujo de un hotfix

Un hotfix es un bug crítico en producción que no puede esperar al próximo sprint.

```
1. Crea rama desde main (no desde develop):
   git checkout main
   git checkout -b hotfix/fix-kafka-connection-timeout

2. Aplica el fix con commits

3. Abre PR hacia main
   → GitHub Actions ejecuta tests automáticos
   → Code review obligatorio (no se omite aunque sea urgente)
   → Merge a main
   → Build + push a ACR con tag sha-<commit>
   → Trigger al ops-repo con environment=production

4. Abre segundo PR hacia develop
   → Para que el fix no se pierda en el próximo sprint
   → Merge a develop
   → Build + push a ACR con tag sha-<commit>
   → Trigger al ops-repo con environment=staging
```

> El hotfix hace merge a **ambas** ramas permanentes. Si solo se mergea a `main`, el fix
> desaparecerá cuando `develop` llegue a producción en el próximo sprint.

### Naming de ramas

- Kebab-case obligatorio: `feature/fix-vote-ui`, `hotfix/worker-panic-nil-pointer`
- Prefijo `feature/` o `hotfix/` obligatorio — el pipeline usa este prefijo para decidir qué jobs ejecutar
- Sin espacios, sin caracteres especiales, sin mayúsculas

### Comportamiento del pipeline por evento

| Evento | Rama origen | Jobs que se ejecutan |
|---|---|---|
| `pull_request` desde `feature/*` | `develop` | Detección de cambios + tests por servicio |
| `pull_request` desde `hotfix/*` | `develop` o `main` | Detección de cambios + tests por servicio |
| `push` | `develop` | Build + push ACR + trigger ops (staging) |
| `push` | `main` | Build + push ACR + trigger ops (production) |

### Versionado de imágenes en ACR

Todas las imágenes usan **tags inmutables**. El tag `latest` no se usa en ningún entorno.

| Evento | Tag generado |
|---|---|
| Push a `develop` | `sha-<commit>` |
| Push a `main` | `sha-<commit>` |

El tag `sha-<commit>` garantiza trazabilidad completa — dado un tag, siempre es posible
identificar el commit exacto que generó esa imagen y reproducir el build.

Las imágenes se publican en ACR (Azure Container Registry), dentro del mismo ecosistema
que el clúster AKS, lo que simplifica la autenticación y reduce la latencia en los pulls.

---

