# Runbook de Demo (Taller)

Este runbook resume la secuencia de demostracion para evidenciar CI/CD end-to-end entre app-repo y ops-repo.

## 1. Verificacion inicial

1. Confirmar acceso al cluster:
   `kubectl get pods -n staging`
2. Confirmar que la infraestructura base esta desplegada (Kafka/PostgreSQL).

## 2. Simular una feature

1. Crear rama desde `develop`:
   `git checkout develop`
   `git checkout -b feature/pruebaviva-1`
2. Aplicar un cambio pequeno en `vote`, `worker` o `result`.

## 3. Ejecutar flujo de integracion

1. Confirmar cambios:
   `git add .`
   `git commit -m "feat(demo): demostracion de integracion continua"`
   `git push origin feature/pruebaviva-1`
2. Abrir PR `feature/* -> develop`.
3. Verificar checks de CI por servicio (tests y validaciones).
4. Aprobar y hacer merge.

## 4. Validar despliegue automatico en staging

1. En GitHub Actions, observar en app-repo:
   `changes -> build-and-push -> trigger-ops-deploy`
2. En ops-repo, observar ejecucion del workflow de despliegue.
3. Confirmar despliegue en cluster:
   `kubectl get pods -n staging`

## 5. Paso a produccion

1. Abrir PR `develop -> main`.
2. Aprobar y hacer merge.
3. Confirmar pipeline equivalente hacia `production`.

## Resultado esperado

- PR de `feature/*` ejecuta CI (sin despliegue).
- Merge a `develop` despliega a `staging`.
- Merge a `main` despliega a `production`.
- Trazabilidad por imagen inmutable `sha-<commit>`.