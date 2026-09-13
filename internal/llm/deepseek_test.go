package llm

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeModel — chaqiruvda nechta option kelganini yozib oladi.
type fakeModel struct {
	gotOpts   int
	gotTools  []*schema.ToolInfo
	withCalls int
}

func (f *fakeModel) Generate(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	f.gotOpts = len(opts)
	return schema.AssistantMessage("ok", nil), nil
}

func (f *fakeModel) Stream(_ context.Context, _ []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	f.gotOpts = len(opts)
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("ok", nil)}), nil
}

func (f *fakeModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	f.gotTools = tools
	f.withCalls++
	return f, nil
}

// Eng muhim xavf: WithTools yangi model qaytaradi — o'ram saqlanmasa
// reasoning_effort tool bog'langandan keyin JIMGINA yo'qoladi (agent doim
// tool bilan ishlaydi, ya'ni daraja hech qachon ketmasdi).
func TestOptionsSurviveWithTools(t *testing.T) {
	f := &fakeModel{}
	var m model.ToolCallingChatModel = &modelWithOptions{
		inner: f,
		opts:  []model.Option{model.WithModel("x")},
	}

	bound, err := m.WithTools([]*schema.ToolInfo{{Name: "t"}})
	if err != nil {
		t.Fatalf("WithTools: %v", err)
	}
	if _, ok := bound.(*modelWithOptions); !ok {
		t.Fatalf("WithTools o'ramni yo'qotdi: %T", bound)
	}

	if _, err := bound.Generate(context.Background(), nil); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if f.gotOpts != 1 {
		t.Errorf("Generate'ga %d option keldi, want 1 (standart option yo'qolgan)", f.gotOpts)
	}

	f.gotOpts = 0
	if _, err := bound.Stream(context.Background(), nil); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if f.gotOpts != 1 {
		t.Errorf("Stream'ga %d option keldi, want 1", f.gotOpts)
	}
}

// Chaqiruvchi option'lari standartga QO'SHILADI (ustidan yozish imkoni qolsin).
func TestOptionsMergeWithCallerOptions(t *testing.T) {
	f := &fakeModel{}
	m := &modelWithOptions{inner: f, opts: []model.Option{model.WithModel("x")}}

	if _, err := m.Generate(context.Background(), nil, model.WithTemperature(0.5)); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if f.gotOpts != 2 {
		t.Errorf("option soni = %d, want 2 (standart + chaqiruvchi)", f.gotOpts)
	}
}
