package grpc

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/session"
	"github.com/muhammadvadud/alora-agent/internal/voice"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const maxVoiceBuffer = 10 << 20

type voiceResponse struct {
	generation int
	event      *alorav1.VoiceEvent
}

type receivedVoiceChunk struct {
	chunk *alorav1.VoiceChunk
	err   error
}

type voiceMeta struct {
	sessionID, userID, tenantID, role, locale, conversationID string
	customerMode                                              bool
}

func (s *Server) VoiceChat(stream grpcgo.BidiStreamingServer[alorav1.VoiceChunk, alorav1.VoiceEvent]) error {
	if s.stt == nil || s.tts == nil {
		return status.Error(codes.Unimplemented, "ovoz xizmati ulanmagan")
	}
	if !s.acquireSession() {
		return status.Error(codes.ResourceExhausted, "agent band (sessiya chegarasi)")
	}
	defer s.active.Add(-1)
	meta, err := voiceMetadata(stream.Context())
	if err != nil {
		return err
	}
	ctx := sessionContext(stream.Context(), meta)

	responses := make(chan voiceResponse, 32)
	var sendMu sync.Mutex
	send := func(event *alorav1.VoiceEvent) error {
		sendMu.Lock()
		defer sendMu.Unlock()
		return stream.Send(event)
	}

	var audio bytes.Buffer
	var format string
	var cancel context.CancelFunc
	generation := 0
	incoming := receiveVoiceChunks(stream)
	inputClosed := false
	for {
		select {
		case <-stream.Context().Done():
			if cancel != nil {
				cancel()
			}
			return stream.Context().Err()
		case result := <-responses:
			if result.generation != generation || result.event == nil {
				continue
			}
			if err := send(result.event); err != nil {
				if cancel != nil {
					cancel()
				}
				return err
			}
			if result.event.Kind == "error" || result.event.Kind == "done" {
				cancel = nil
				if inputClosed {
					return nil
				}
			}
		case received := <-incoming:
			chunk, recvErr := received.chunk, received.err
			if recvErr != nil {
				if errors.Is(recvErr, io.EOF) {
					inputClosed = true
					incoming = nil
					if cancel == nil {
						return nil
					}
					continue
				}
				if cancel != nil {
					cancel()
				}
				return recvErr
			}
			if chunk != nil {
				if cancel != nil {
					cancel()
					cancel = nil
					generation++
					audio.Reset()
				}
				if len(chunk.GetAudioData())+audio.Len() > maxVoiceBuffer {
					return status.Error(codes.ResourceExhausted, "audio buferi 10 MB dan oshdi")
				}
				audio.Write(chunk.GetAudioData())
				if chunk.GetFormat() != "" {
					format = chunk.GetFormat()
				}
				if !chunk.GetIsFinal() {
					continue
				}
				generation++
				jobCtx, jobCancel := context.WithCancel(ctx)
				cancel = jobCancel
				data := append([]byte(nil), audio.Bytes()...)
				jobFormat := format
				audio.Reset()
				go s.voiceResponse(jobCtx, generation, meta.sessionID, data, jobFormat, responses)
			}
		}
	}
}

// receiveVoiceChunk alohida gorutinada turadi, chunki agent/TTS oqimi paytida
// Recv bloklanib qolsa barge-in yangi audio'ni vaqtida bekor qila olmaydi.
func receiveVoiceChunks(stream grpcgo.BidiStreamingServer[alorav1.VoiceChunk, alorav1.VoiceEvent]) <-chan receivedVoiceChunk {
	incoming := make(chan receivedVoiceChunk, 1)
	go func() {
		defer close(incoming)
		for {
			chunk, err := stream.Recv()
			if err != nil {
				incoming <- receivedVoiceChunk{err: err}
				return
			}
			select {
			case incoming <- receivedVoiceChunk{chunk: chunk}:
			case <-stream.Context().Done():
				return
			}
		}
	}()
	return incoming
}

func (s *Server) voiceResponse(ctx context.Context, generation int, sessionID string, data []byte, format string, out chan<- voiceResponse) {
	result := func(event *alorav1.VoiceEvent) bool {
		select {
		case out <- voiceResponse{generation: generation, event: event}:
			return true
		case <-ctx.Done():
			return false
		}
	}
	sttText, sttUsage, err := s.stt.Transcribe(ctx, bytes.NewReader(data), format, s.sttLang)
	if err != nil {
		result(&alorav1.VoiceEvent{Kind: "error", Error: "ovozni tanib bo'lmadi", ChatEvent: &alorav1.ChatEvent{Kind: string(agent.KindError), Tool: "stt", Error: "ovozni tanib bo'lmadi"}})
		return
	}
	if err := validateMessage(sttText, s.maxMessageChars); err != nil {
		result(&alorav1.VoiceEvent{Kind: "error", Error: "xabar juda uzun yoki bo'sh", ChatEvent: &alorav1.ChatEvent{Kind: string(agent.KindError), Tool: "stt", Error: err.Error()}})
		return
	}
	result(&alorav1.VoiceEvent{Kind: "text", Content: sttText, ChatEvent: &alorav1.ChatEvent{Kind: "transcript", Content: sttText}})

	if metrics := s.metrics; metrics != nil {
		metrics.VoiceSTTSeconds.Observe(sttUsage.STTSeconds)
	}
	stream := s.chat.Stream(ctx, sessionID, sttText)
	splitter := voice.SentenceSplitter{MaxChars: 120}
	var ttsWG sync.WaitGroup
	var usageMu sync.Mutex
	var ttsUsage voice.Usage
	synthesize := func(sentence string) {
		ttsWG.Add(1)
		go func() {
			defer ttsWG.Done()
			text := voice.NormalizeForTTS(sentence, s.sttLang)
			audio, audioFormat, usage, synthErr := s.tts.Synthesize(ctx, text, s.ttsVoice)
			if synthErr != nil {
				slog.Warn("tts xatosi, matn oqimi davom etadi", "error", synthErr)
				return
			}
			usageMu.Lock()
			ttsUsage.STTSeconds += usage.STTSeconds
			ttsUsage.TTSChars += usage.TTSChars
			usageMu.Unlock()
			if metrics := s.metrics; metrics != nil {
				metrics.VoiceTTSChars.Add(float64(usage.TTSChars))
			}
			for chunk := range audio {
				if !result(&alorav1.VoiceEvent{Kind: "audio", AudioData: chunk, AudioFormat: audioFormat}) {
					return
				}
			}
		}()
	}
	for event := range stream {
		if event.Kind == agent.KindThinking {
			continue
		}
		mapped := toProto(event)
		// done TTS yakunlangach yuboriladi; aks holda klient oxirgi audio
		// kelishidan oldin suhbat tugagan deb hisoblab qo'yadi.
		if event.Kind != agent.KindDone && mapped != nil && !result(&alorav1.VoiceEvent{Kind: "text", Content: mapped.Content, ChatEvent: mapped}) {
			return
		}
		if event.Kind == agent.KindContent {
			for _, sentence := range splitter.Push(event.Content) {
				synthesize(sentence)
			}
		}
		if event.Kind == agent.KindDone {
			if sentence := splitter.Flush(); sentence != "" {
				synthesize(sentence)
			}
			ttsWG.Wait()
			if mapped != nil && mapped.Usage != nil {
				mapped.Usage.SttSeconds = float32(sttUsage.STTSeconds)
				mapped.Usage.TtsChars = int32(ttsUsage.TTSChars)
			}
			result(&alorav1.VoiceEvent{Kind: "done", ChatEvent: mapped})
			return
		}
	}
}

func voiceMetadata(ctx context.Context) (voiceMeta, error) {
	get := func(key string) string {
		values := metadata.ValueFromIncomingContext(ctx, key)
		if len(values) == 0 {
			return ""
		}
		return values[0]
	}
	meta := voiceMeta{sessionID: get("x-session-id"), userID: get("x-user-id"), tenantID: get("x-tenant-id"), role: get("x-role"), locale: get("x-locale"), conversationID: get("x-conversation-id"), customerMode: get("x-customer-mode") == "1"}
	if meta.sessionID == "" || meta.userID == "" || meta.tenantID == "" || meta.role == "" || meta.locale == "" || meta.conversationID == "" || get("x-customer-mode") == "" {
		return voiceMeta{}, status.Error(codes.InvalidArgument, "ovoz sessiyasi metadata'si to'liq emas")
	}
	return meta, nil
}

func sessionContext(ctx context.Context, meta voiceMeta) context.Context {
	ctx = session.WithID(ctx, meta.sessionID)
	return curtains.WithUser(ctx, curtains.UserInfo{UserID: meta.userID, TenantID: meta.tenantID, Role: curtains.UserRole(meta.role), Locale: meta.locale, CustomerMode: meta.customerMode, ConversationID: meta.conversationID})
}
