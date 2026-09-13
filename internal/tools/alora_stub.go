package tools

import "github.com/cloudwego/eino/components/tool"

// Vaqtinchalik stub'lar — haqiqiy adapterlar 1.19–1.21 va 2.15 da yoziladi (shu to'lqinda).
// nil qaytarish agent.NewChat da xato beradi, shuning uchun bo'sh ro'yxatda ishlatilmaydi —
// main.go ulanguncha Alora() chaqirilmaydi.
func ProductSearch(Deps) tool.BaseTool    { return nil }
func CurtainCalculate(Deps) tool.BaseTool { return nil }
func PriceQuote(Deps) tool.BaseTool       { return nil }
func SetContext(Deps) tool.BaseTool       { return nil }
