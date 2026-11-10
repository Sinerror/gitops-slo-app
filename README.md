# gitops-slo-app

Код сервисов и CI для проекта **[gitops-slo-platform](https://github.com/Sinerror/gitops-slo-platform)**
(перевод микросервисного приложения на GitOps с SLO). English: [README.en.md](README.en.md).

Приложение — [Online Boutique](https://github.com/GoogleCloudPlatform/microservices-demo) v0.10.7
(Apache-2.0). Почти все сервисы берутся готовыми образами апстрима; здесь лежит только то,
что собирается своим CI:

| Сервис | Что это |
| --- | --- |
| `src/checkoutservice` | форк апстрима: после оформления заказа публикует событие в Kafka (`events.go`, асинхронно — Kafka не влияет на оформление), плюс управляемая ошибка `FAULT_ERROR_RATE` для учебной аварии «плохой релиз» (`faults.go`) |
| `src/order-consumer` | свой сервис на Go: читает топик `orders`, метрики Prometheus, `PROCESSING_DELAY_MS` для сценария «отставание consumer» |

## Пайплайн

```
push в main ─► test (gofmt, vet, go test -race) ─► build ─► Trivy (HIGH/CRITICAL) ─► push в GHCR
                                                                   │ упал — образ не публикуется
                                                                   ▼
                          коммит тега sha-xxxxxxx в gitops-slo-platform/envs/staging ─► ArgoCD ─► smoke-тест
workflow promote (вручную) ─► тот же тег в envs/prod ─► ArgoCD ─► smoke-тест
```

- Actions закреплены по SHA коммита; Trivy запускается из официального образа по digest
  (`aquasecurity/trivy-action` не используется после его компрометации).
- В коммит gitops-репозитория пишутся трейлеры `Source-Commit` и `Source-Commit-Time` —
  по ним `scripts/dora.sh` в платформенном репозитории считает lead time.
- Гейт Trivy (та же команда, что в CI, прогнана локально до первого пуша) не пропустил исходные образы
  на Go 1.27.0: 7 и 4 HIGH CVE (stdlib, `x/net`, `klauspost/compress`); после обновления builder
  до Go 1.27.3 и зависимостей — 0.

## Настройка (один раз)

1. Репозиторий публичный; пакеты в GHCR после первой публикации сделать публичными
   (Package settings → Change visibility), иначе kind не сможет скачать образы.
2. Секрет `GITOPS_TOKEN`: fine-grained PAT с доступом **только** к `Sinerror/gitops-slo-platform`,
   права *Contents: Read and write*. Settings → Secrets and variables → Actions.

## Локально

```bash
cd src/order-consumer && go test ./...
cd src/checkoutservice && go test -vet=off ./...   # vet: замечания в коде апстрима
```
