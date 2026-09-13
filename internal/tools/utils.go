package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// Calculator — ikki son ustida arifmetik amal bajaruvchi trivial tool. Struct'lar: dto.go.
func Calculator() tool.BaseTool {
	return must(utils.InferTool("calculator",
		"Ikki son ustida arifmetik amal bajaradi. op: add, sub, mul, div.", calculate))
}

// Clock — berilgan IANA vaqt mintaqasidagi joriy vaqtni qaytaruvchi trivial tool.
func Clock() tool.BaseTool {
	return must(utils.InferTool("current_time",
		"Berilgan IANA vaqt mintaqasidagi joriy vaqtni qaytaradi (masalan Asia/Tashkent).", currentTime))
}

func calculate(_ context.Context, req calcArgs) (float64, error) {
	switch req.Op {
	case "add":
		return req.A + req.B, nil
	case "sub":
		return req.A - req.B, nil
	case "mul":
		return req.A * req.B, nil
	case "div":
		if req.B == 0 {
			return 0, fmt.Errorf("nolga bo'lish mumkin emas")
		}
		return req.A / req.B, nil
	default:
		return 0, fmt.Errorf("noma'lum amal %q (add, sub, mul, div)", req.Op)
	}
}

func currentTime(_ context.Context, req timeArgs) (string, error) {
	tz := req.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return "", fmt.Errorf("noto'g'ri vaqt mintaqasi: %s", tz)
	}
	return time.Now().In(loc).Format("2006-01-02 15:04:05 MST"), nil
}
