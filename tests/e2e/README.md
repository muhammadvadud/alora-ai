# Django SSE E2E

Test real DeepSeek provider orqali Django SSE gateway → Go agent → tool'lar →
Django Pricing → SSE oqimini tekshiradi. Mock provider ishlatilmaydi.

Ishga tushirishdan oldin Django `runserver` (HTTP `8000`), Django gRPC (`9091`)
va Go agent (`9095`) ishlayotgan bo‘lsin. Agent loyiha ildizidan ishga tushiriladi,
shunda `.env` agentning odatiy yuklash oqimi bilan o‘qiladi. API kalitlari bu
hujjatga yozilmaydi.

```bash
cd projects/backend
.venv/bin/python manage.py runserver 8000

cd projects/ai
ALORA_BACKEND_ADDR=localhost:9091 \
AGENT_SERVICE_TOKEN=devtoken \
go test -tags live ./tests/e2e -run TestDjangoSSE_AgentEndToEnd -v
```

JWT avtomatik olinishi uchun `ALORA_E2E_PHONE` va `ALORA_E2E_PASSWORD` bering.
Tayyor JWT yoki suhbatni qayta ishlatish mumkin: `ALORA_E2E_JWT` va
`ALORA_E2E_CONVERSATION_ID`. `ALORA_E2E_DJANGO` bilan Django URL'ini almashtirish
mumkin.

Test quyidagilarni kutadi: birinchi xabarda `set_context` yoki `product_search`,
`products`, `content` va `done.usage`; ikkinchi xabarda `curtain_calculate` va
matndagi barcha so‘m summalari tool natijasidan kelishi; xabarlar hamda Django
usage summary yozuvi 5 soniya ichida saqlanishi.
