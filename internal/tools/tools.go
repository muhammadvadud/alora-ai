// Package tools agent uchun tool'larni beradi (yupqa adapter qatlami).
//
// Tuzilma: dto.go — kirish/chiqish struct'lari; <feature>.go — handler mantiq va
// tool konstruktorlari. Registratsiya main.go da (composition root) — barcha tool
// bir joyda ko'rinadi. Domen mantiq alohida paketda (internal/curtains).
package tools

import (
	"github.com/cloudwego/eino/components/tool"
)

// Deps — feature tool'lari uchun bog'liqliklar (main tomonidan beriladi).
// Alora bog'liqliklari (Catalog, State, Backend, Clock) tasks.md 1.17 da qo'shiladi.
type Deps struct{}

// must — InferTool xatosi handler imzosidagi DASTURLASH xatosi (runtime'da yuz bermaydi),
// shuning uchun konstruktorlarni soddalashtirish uchun panic qilamiz (regexp.MustCompile uslubi).
func must(t tool.BaseTool, err error) tool.BaseTool {
	if err != nil {
		panic("tools: noto'g'ri tool ta'rifi: " + err.Error())
	}
	return t
}
