# Alora Agent Monitoring (Grafana & Prometheus)

Ushbu papkada Alora Agent (Go) uchun monitoring dashboard va alert qoidalari joylashgan. Alert va metrika thresholdlari loyihaning `rules.md §13.3` bo'limiga asoslangan.

## 1. Dashboard import qilish (Grafana)
1. Grafana web panelini oching (masalan, `http://localhost:3000`).
2. Chap menyudan **Dashboards** -> **Import** tugmasini bosing.
3. `alora-agent.json` faylini yuklang yoki JSON kodini nusxalab qo'ying.
4. **Datasource** sifatida o'zingizning Prometheus ulanishingizni (`${DS_PROMETHEUS}`) tanlang.

## 2. Alert qoidalarini qo'llash (Prometheus)
Agar Kubernetes dan foydalansangiz va `prometheus-operator` o'rnatilgan bo'lsa, qoidalarni quyidagi buyruq bilan qo'llang:
```bash
kubectl apply -f prometheus-rules.yaml
```
Agar standart Prometheus ishlatsangiz, `prometheus-rules.yaml` faylidagi `.spec.groups` qismini Prometheus konfigining `rule_files` bo'limiga ulang.

## 3. Thresholdlar va qoidalar
Barcha qoidalar `rules.md §13.3` dagi qat'iy talablardan olingan:
- **Katalog Stale**: `age_seconds > 600` yoki `stale > 0` (10 daqiqalik threshold).
- **Outbox Dead**: Retry limiti tugagan yozuvlar noldan oshsa `Critical`.
- **TTFT (Time To First Token)**: p95 uchun chegara 2 soniya.
- **LLM xatolar ulushi**: Asosiy API xatoliklari 5% dan oshsa alert.
- **Watch uzilishi**: Oqim > 1 daqiqa uzilsa (stat = 0).
