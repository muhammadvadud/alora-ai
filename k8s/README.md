# Alora AI Agent — Kubernetes (k8s/agent.yaml)

K8s manifestlari (tasks.md 4.7, rules.md §15/§16/§17). Bitta fayl, `---` bilan ajratilgan
resurslar: `Namespace`, `ConfigMap`, `Secret`, `Deployment`, `Service`, `HPA`, `PDB`,
`ServiceMonitor`, `NetworkPolicy`.

## Qo'llash tartibi

```bash
# 1) Namespace + asosiy resurslar (Secret bo'sh namuna sifatida keladi).
kubectl apply -f k8s/agent.yaml

# 2) Haqiqiy maxfiy qiymatlar — yaml'ga EMAS. sealed-secrets/vault orqali:
kubeseal --format yaml < alora-agent-secrets.yaml > ../sealed-secret.yaml
kubectl apply -f ../sealed-secret.yaml
#    yoki Vault Agent Injector bo'lsa: Deployment'ga annotation qo'shiladi.
#    Secret o'zgartirilsa Deployment restart:  kubectl rollout restart deploy/alora-agent

# 3) Holatni kuzatish
kubectl get pods -n alora -w
kubectl get hpa,pdb -n alora
```

> **Diqqat:** Secret'dagi bo'sh qiymatlar real Opaque secretni o'chirib tashlamaydi —
> `kubectl apply` bo'sh qiymatlarni yozadi. Shuning uchun prod'da **avval** sealed secret
> qo'llanadi, yoki bo'sh namuna keyin o'chiriladi (`kubectl delete secret alora-agent-secrets`).
> Kalitlar: `DEEPSEEK_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `ELEVENLABS_API_KEY`,
> `AGENT_SERVICE_TOKEN`, `DATABASE_URL` (rules.md §12.2 ro'yxati bilan mos).

## TAG almashtirish

Image: `ghcr.io/muhammadvadud/alora-ai:${TAG}` — CI/kustomize almashtiradi.

```bash
# CI (kustomize):
kubectl set image deploy/alora-agent \
  alora-agent=ghcr.io/muhammadvadud/alora-ai:v0.1.0 -n alora

# qo'lda (sed + apply):
sed "s|\${TAG}|v0.1.0|" k8s/agent.yaml | kubectl apply -f -
```

## Rollout / rollback

```bash
kubectl rollout status deploy/alora-agent -n alora     # tugashini kuzatish
kubectl rollout history deploy/alora-agent -n alora    # reviziyalar
kubectl rollout undo deploy/alora-agent -n alora       # avvalgi reviziyaga qaytish
kubectl rollout undo deploy/alora-agent --to-revision=2 -n alora
```

RollingUpdate `maxUnavailable: 0, maxSurge: 1` — har yangilanishda eski pod to'liq
tayyor bo'lguncha yangisi boshlanmaydi, xizmat uzilmaydi.

## Probe'lar mantig'i

- **`startupProbe` /healthz:8081** — `failureThreshold: 30` (300s). Ishga tushishda katalog
  snapshot Django'dan yuklanadi (10k mahsulot ~10s); shu muddatda pod `NotReady` bo'lishi normal.
- **`readinessProbe` /readyz:8081 (5s)** — `/readyz` katalog snapshot **tugamaguncha 503**
  qaytaradi (rules.md §4.1.3). Shunda pod Service'dan chiqariladi, `Chat` so'rovlari UDP
  `UNAVAILABLE` oladi. Snapshot qayta boshlanganda pod yana ready bo'ladi.
  Katalog **stale** (10m+) holda esa `/readyz` o'chmaydi — `Health` gRPC'da `degraded`
  bo'ladi (stale — xato emas, ogohlantirish). `/readyz` 503 bo'lsa `alora_catalog_*`
  metrikalari va `catalog_resync` loglarini tekshiring (docs/RUNBOOK.md).
- **`livenessProbe` /healthz:8081 (10s, 3 marta)** — agent qulagan/o'lik holda pod qayta
  boshlanadi. `/healthz` server jarayonining o'zi turadi (xizmat holatiga bog'liq emas).

## HPA sababi

- **min 2 / max 5**, CPU 70% yoki mem 80% oshsa ko'payadi. 1 instans ~50 parallel sessiya
  sig'imida (`grpc.max_sessions: 100`, rules.md §15) — asosiy resurs sarfi LLM stream va
  katalog xotirasi. CPU kechikishi 1.5s TTFT byudjetini buzmaslik uchun 70% da oldin ko'taradi.
- `scaleDown` stabilizatsiya **300s** — shpiksdan keyin keraksiz tez qisqarish bo'lmaydi.
- PDB `minAvailable: 1` — oralatishlarda (node drain) kamida 1 instans xizmat beradi.

## ServiceMonitor / NetworkPolicy eslatmalari

- `ServiceMonitor` faqat **Prometheus Operator** bo'lsa kerak; `/metrics` :8081, 15s interval.
  Label `release: prometheus` Prometheus CR ning `serviceMonitorSelector`'iga mos kelmasa
  tahrirlang. Grafana dashboard: `docs/grafana/alora-agent.json`.
- `NetworkPolicy` — egress `0.0.0.0/0:443` faqat LLM API'lari uchun (HTTPS):
  DeepSeek/Anthropic/OpenAI/ElevenLabs. Django gRPC `app=alora-backend:50051`, Postgres
  `app=postgres:5432` label'larini real deployment bilan moslashtiring.
  **CNI kubelet probe trafigini policy orqali o'tkazmasa**, `/healthz`/`/readyz` tekshiruvlari
  bloklanadi — shunda node CIDR'ini ingress ruxsatiga qo'shish kerak:
  `from: [{ ipBlock: { cidr: <node-node-net-cidr> } }]`.