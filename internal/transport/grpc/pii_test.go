package grpc

import (
	"context"
	"strings"
	"testing"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/agent"
)

// B6 (ADR-020): telefon LLM kontekstiga KIRMAYDI, lekin Django LogMessage da
// ASL matn bilan qoladi.
//
// 2026-09-28 live: mijoz telefonini yozdi, model uni tool'ga urinib yuboradi
// (order_draft ga PII maydonlari qo'shildi) va telefon katalog/CRM loglarida
// qoldi. Endi: (1) order_draft da PII argumentlari yo'q, (2) transport LLM'ga
// yuborishdan OLDIN telefon maskalaydi, (3) audit logi o'zgar maydonini oladi.

type capturingStreamer struct {
	events  []agent.Event
	mInputs []string
}

func (c *capturingStreamer) Stream(ctx context.Context, _, input string) <-chan agent.Event {
	c.mInputs = append(c.mInputs, input)
	out := make(chan agent.Event, len(c.events))
	go func() {
		defer close(out)
		for _, event := range c.events {
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (c *capturingStreamer) Reset(context.Context, string) error { return nil }

// B6: LLM ga masksiz telefon ketmasligi kerak.
func TestChat_ScrubsPhoneBeforeLLM(t *testing.T) {
	for _, msg := range []string{
		"Telefonim +998901234567, qo'ng'iroq qiling",
		"mening raqamim 901234567",
		"+998 90 123 45 67",
		"90-123-45-67",
	} {
		reporter := &recordingReporter{}
		fake := &capturingStreamer{events: []agent.Event{{Kind: agent.KindContent, Content: "raqam qabul qilindi"}, {Kind: agent.KindDone}}}
		srv := New(fake, Options{Reporter: reporter})
		stream := &fakeChatStream{ctx: context.Background()}
		req := &alorav1.ChatRequest{SessionId: "s1", UserId: "u1", TenantId: "t1", Message: msg}
		if err := srv.Chat(req, stream); err != nil {
			t.Fatal(err)
		}
		if len(fake.mInputs) != 1 {
			t.Fatalf("LLM chaqiruvi soni: %d", len(fake.mInputs))
		}
		got := fake.mInputs[0]
		if strings.Contains(got, "1234567") {
			t.Errorf("msg=%q: telefon LLM kontekstiga o'tdi: %q", msg, got)
		}
		if !strings.Contains(got, piiMarker) {
			t.Errorf("msg=%q: maskovchi belgi yo'q: %q", msg, got)
		}

		// Django LogMessage ASL matnni olishi kerak — mijoz o'z raqamini ko'radi
		if len(reporter.logs) == 0 || reporter.logs[0].Content != msg {
			t.Errorf("msg=%q: audit logi asl matnni olmadi: %#v", msg, reporter.logs)
		}
	}
}

// piiMarker — pii.mask.go dagi maskovchi belgi (test uni qayta ishlatmasin).
const piiMarker = "***"

func TestChat_ScrubsLabeledNameAddressAndEmailBeforeLLM(t *testing.T) {
	message := "Mening ismim Sinovbek Auditov; email sinovbek@example.test; manzilim Sinov ko'chasi 123"
	reporter := &recordingReporter{}
	fake := &capturingStreamer{events: []agent.Event{{Kind: agent.KindDone}}}
	srv := New(fake, Options{Reporter: reporter})
	stream := &fakeChatStream{ctx: context.Background()}
	err := srv.Chat(&alorav1.ChatRequest{SessionId: "private", UserId: "u1", TenantId: "t1", Message: message}, stream)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.mInputs) != 1 {
		t.Fatalf("model kirishi: %v", fake.mInputs)
	}
	for _, secret := range []string{"Sinovbek", "sinovbek@example.test", "Sinov ko'chasi"} {
		if strings.Contains(fake.mInputs[0], secret) {
			t.Fatalf("LLMga PII o'tdi: %s", fake.mInputs[0])
		}
	}
	if len(reporter.logs) == 0 || reporter.logs[0].Content != message {
		t.Fatal("Django asl xabarni audit uchun olishi kerak")
	}
}

// B6: PII yo'q xabarlar o'zgarmasligi kerak (keraksiz maskalash sessiyani iflos qiladi).
func TestChat_NoPhoneLeavesMessageIntact(t *testing.T) {
	for _, msg := range []string{"salom", "parda 4 m bo'libchi", "mato rangini ko'rsat"} {
		fake := &capturingStreamer{events: []agent.Event{{Kind: agent.KindDone}}}
		srv := New(fake, Options{})
		stream := &fakeChatStream{ctx: context.Background()}
		if err := srv.Chat(&alorav1.ChatRequest{SessionId: "s1", UserId: "u1", TenantId: "t1", Message: msg}, stream); err != nil {
			t.Fatal(err)
		}
		if len(fake.mInputs) != 1 || fake.mInputs[0] != msg {
			t.Errorf("PII yo'q xabar o'zgardi: %q != %q", fake.mInputs, msg)
		}
	}
}
