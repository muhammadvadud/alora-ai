package grpc

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/voice"
	"google.golang.org/grpc/metadata"
)

type fakeVoiceStream struct {
	ctx    context.Context
	chunks chan *alorav1.VoiceChunk
	mu     sync.Mutex
	events []*alorav1.VoiceEvent
}

func (s *fakeVoiceStream) Recv() (*alorav1.VoiceChunk, error) {
	select {
	case chunk, ok := <-s.chunks:
		if !ok {
			return nil, io.EOF
		}
		return chunk, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}
func (s *fakeVoiceStream) Send(event *alorav1.VoiceEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}
func (s *fakeVoiceStream) SetHeader(metadata.MD) error  { return nil }
func (s *fakeVoiceStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeVoiceStream) SetTrailer(metadata.MD)       {}
func (s *fakeVoiceStream) Context() context.Context     { return s.ctx }
func (s *fakeVoiceStream) SendMsg(any) error            { return nil }
func (s *fakeVoiceStream) RecvMsg(any) error            { return nil }

type fakeVoiceSTT struct {
	text  string
	err   error
	calls int
}

func (s *fakeVoiceSTT) Transcribe(context.Context, io.Reader, string, string) (string, voice.Usage, error) {
	s.calls++
	return s.text, voice.Usage{STTSeconds: .1}, s.err
}

type fakeVoiceTTS struct {
	mu       sync.Mutex
	texts    []string
	started  chan struct{}
	canceled chan struct{}
}

func (s *fakeVoiceTTS) Synthesize(ctx context.Context, text, _ string) (<-chan []byte, string, voice.Usage, error) {
	s.mu.Lock()
	s.texts = append(s.texts, text)
	s.mu.Unlock()
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	out := make(chan []byte, 2)
	go func() {
		defer close(out)
		select {
		case out <- []byte("a1"):
		case <-ctx.Done():
			if s.canceled != nil {
				select {
				case s.canceled <- struct{}{}:
				default:
				}
			}
			return
		}
		select {
		case out <- []byte("a2"):
		case <-ctx.Done():
			if s.canceled != nil {
				select {
				case s.canceled <- struct{}{}:
				default:
				}
			}
		}
	}()
	return out, "pcm", voice.Usage{TTSChars: len([]rune(text))}, nil
}

type fakeVoiceChat struct{ events []agent.Event }

func (f fakeVoiceChat) Stream(ctx context.Context, _, _ string) <-chan agent.Event {
	out := make(chan agent.Event, len(f.events))
	go func() {
		defer close(out)
		for _, event := range f.events {
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
func (fakeVoiceChat) Reset(context.Context, string) error { return nil }

func voiceTestServer(t *testing.T, stt *fakeVoiceSTT, tts *fakeVoiceTTS, chunks chan *alorav1.VoiceChunk) (*Server, *fakeVoiceStream, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-session-id", "s1", "x-user-id", "u1", "x-tenant-id", "t1", "x-role", "employee", "x-locale", "uz", "x-customer-mode", "0", "x-conversation-id", "c1")))
	stream := &fakeVoiceStream{ctx: ctx, chunks: chunks}
	server := New(fakeVoiceChat{events: []agent.Event{{Kind: agent.KindStart}, {Kind: agent.KindContent, Content: "Narx 2 500 000 so'm."}, {Kind: agent.KindDone}}}, Options{STT: stt, TTS: tts})
	return server, stream, cancel
}

func TestVoiceChatTranscriptAudioDoneAndNormalization(t *testing.T) {
	stt := &fakeVoiceSTT{text: "narxni ayting"}
	tts := &fakeVoiceTTS{}
	chunks := make(chan *alorav1.VoiceChunk, 1)
	chunks <- &alorav1.VoiceChunk{AudioData: []byte("audio"), Format: "pcm_s16le", IsFinal: true}
	close(chunks)
	srv, stream, cancel := voiceTestServer(t, stt, tts, chunks)
	defer cancel()
	if err := srv.VoiceChat(stream); err != nil {
		t.Fatal(err)
	}
	if len(stream.events) < 5 || stream.events[0].GetChatEvent().GetKind() != "transcript" || stream.events[len(stream.events)-1].GetKind() != "done" {
		t.Fatalf("events = %#v", stream.events)
	}
	if len(tts.texts) != 1 || tts.texts[0] != "Narx ikki million besh yuz ming so'm." {
		t.Fatalf("tts text = %#v", tts.texts)
	}
}

func TestVoiceChatBargeInCancelsTTS(t *testing.T) {
	stt := &fakeVoiceSTT{text: "birinchi"}
	tts := &fakeVoiceTTS{started: make(chan struct{}, 2), canceled: make(chan struct{}, 1)}
	chunks := make(chan *alorav1.VoiceChunk, 2)
	chunks <- &alorav1.VoiceChunk{AudioData: []byte("one"), Format: "pcm", IsFinal: true}
	go func() {
		time.Sleep(10 * time.Millisecond)
		chunks <- &alorav1.VoiceChunk{AudioData: []byte("two"), Format: "pcm", IsFinal: true}
		close(chunks)
	}()
	srv, stream, cancel := voiceTestServer(t, stt, tts, chunks)
	defer cancel()
	if err := srv.VoiceChat(stream); err != nil {
		t.Fatal(err)
	}
	if len(tts.texts) < 1 {
		t.Fatal("TTS chaqirilmadi")
	}
}

func TestVoiceChatSTTErrorKeepsStreamAlive(t *testing.T) {
	stt := &fakeVoiceSTT{err: io.ErrUnexpectedEOF}
	tts := &fakeVoiceTTS{}
	chunks := make(chan *alorav1.VoiceChunk, 1)
	chunks <- &alorav1.VoiceChunk{AudioData: []byte("bad"), Format: "pcm", IsFinal: true}
	close(chunks)
	srv, stream, cancel := voiceTestServer(t, stt, tts, chunks)
	defer cancel()
	if err := srv.VoiceChat(stream); err != nil {
		t.Fatal(err)
	}
	if len(stream.events) != 1 || stream.events[0].GetKind() != "error" || stream.events[0].GetChatEvent().GetTool() != "stt" {
		t.Fatalf("events = %#v", stream.events)
	}
}
