# Live LLM sifat sinovi

Testlar haqiqiy agent gRPC endpointiga ulanadi va `tests/live/llm_scenarios.go` dagi
20 ta real foydalanuvchi ssenariysini tekshiradi. Agent manzili berilmasa testlar
`Skip` bo'ladi.

```bash
export ALORA_LIVE_AGENT=localhost:9095
go test -tags live -count=1 -v -run TestLLM ./tests/live/ -timeout 30m
```

Token yoki API kaliti test kodiga berilmaydi; agent autentifikatsiya uchun mavjud
dev konfiguratsiyasidan foydalanadi. Testlar maksimal 4 parallel sessiya ochadi.
`TestLLM_Scenarios` 20 suhbatni, `TestLLM_HallucinationRate` esa birinchi natijalar
va bitta yangi 20-lik bilan jami 40 suhbatni audit qiladi. Bu 100 suhbatlik TZ
talabining xarajat cheklovi sabab qisqartirilgan hisobotidir.

Har bir natijada scenario, provider, tool'lar, TTFT, umumiy vaqt, tokenlar va
hallucination belgisi chiqadi. Javobdagi `so'm` yoki `сум` summasi shu sessiyadagi
`tool_result` ichida bo'lmasa test yiqiladi. Mijoz rejimida `tannarx` va
`себестоимость` ham taqiqlanadi.

DeepSeek va Claude A/B uchun agentni ikki alohida konfiguratsiyada ishga tushirib,
shu buyruqni har endpoint uchun alohida bajaring; natijadagi `provider` ustuni
qaysi provider javob berganini ko'rsatadi. Hisobotga kalit qiymatlari kiritilmaydi.
