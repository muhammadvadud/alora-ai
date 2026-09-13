package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// SetContext — model aniqlagan mijoz ehtiyojini sessiya holatiga yozadi (ADR-003).
// Nega: holat model kontekstiga emas, state'ga tayanadi (TZ §5) — trimHistory'da yo'qolmaydi.
func SetContext(d Deps) tool.BaseTool {
	h := &setContextHandler{d: d}
	return must(utils.InferTool("set_context",
		"Mijoz ehtiyoji aniqlanganda (xona, uslub, yorug'lik, rang, konstruksiya, budjet) shu tool bilan yoz. "+
			"Natijada keyingi so'raladigan savol keladi — bir vaqtda BITTA savol ber.",
		h.do))
}

type setContextHandler struct{ d Deps }

func (h *setContextHandler) do(ctx context.Context, a setContextArgs) (setContextResult, error) {
	st, sid := loadState(ctx, h.d)
	st.Apply("set_context", argsMap(a), now(h.d))
	saveState(ctx, h.d, sid, st)
	_, hint := st.NextQuestion()
	return setContextResult{State: st.Summary(), NextQuestion: hint}, nil
}
