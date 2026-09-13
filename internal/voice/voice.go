package voice

import (
	"context"
	"fmt"
	"io"
	"sort"
)

// Usage STT va TTS tizimlarining ishlash statistikasini saqlaydi.
type Usage struct {
	STTSeconds float64
	TTSChars   int
}

// STT - Speech-to-Text interfeysi.
type STT interface {
	Transcribe(ctx context.Context, audio io.Reader, format string, lang string) (text string, u Usage, err error)
}

// TTS - Text-to-Speech interfeysi.
// Synthesize qilingan audio kanali orqali qaytariladi, kanal ctx bekor bo'lganda yopiladi.
type TTS interface {
	Synthesize(ctx context.Context, text, voice string) (audio <-chan []byte, format string, u Usage, err error)
}

// STTConfig STT provayderi uchun sozlamalar.
type STTConfig struct {
	Provider string
	Language string
	APIKey   string
}

// TTSConfig TTS provayderi uchun sozlamalar.
type TTSConfig struct {
	Provider string
	Voice    string
	Speed    float64
	APIKey   string
}

// STTBuilder STT provayderini yaratuvchi funksiya turi.
type STTBuilder func(ctx context.Context, cfg STTConfig) (STT, error)

// TTSBuilder TTS provayderini yaratuvchi funksiya turi.
type TTSBuilder func(ctx context.Context, cfg TTSConfig) (TTS, error)

var (
	sttRegistry = map[string]STTBuilder{}
	ttsRegistry = map[string]TTSBuilder{}
)

// RegisterSTT yangi STT provayderini ro'yxatdan o'tkazadi.
// Agar provayder oldin ro'yxatdan o'tgan bo'lsa, vahima (panic) ko'tariladi.
func RegisterSTT(name string, b STTBuilder) {
	if _, exists := sttRegistry[name]; exists {
		panic(fmt.Sprintf("stt provider %q ro'yxatda allaqachon mavjud", name))
	}
	sttRegistry[name] = b
}

// RegisterTTS yangi TTS provayderini ro'yxatdan o'tkazadi.
// Agar provayder oldin ro'yxatdan o'tgan bo'lsa, vahima (panic) ko'tariladi.
func RegisterTTS(name string, b TTSBuilder) {
	if _, exists := ttsRegistry[name]; exists {
		panic(fmt.Sprintf("tts provider %q ro'yxatda allaqachon mavjud", name))
	}
	ttsRegistry[name] = b
}

// NewSTT berilgan sozlamalar asosida STT provayderini yaratadi.
func NewSTT(ctx context.Context, cfg STTConfig) (STT, error) {
	build, ok := sttRegistry[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("stt provider %q ro'yxatda yo'q (mavjud: %v)", cfg.Provider, ProvidersSTT())
	}
	return build(ctx, cfg)
}

// NewTTS berilgan sozlamalar asosida TTS provayderini yaratadi.
func NewTTS(ctx context.Context, cfg TTSConfig) (TTS, error) {
	build, ok := ttsRegistry[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("tts provider %q ro'yxatda yo'q (mavjud: %v)", cfg.Provider, ProvidersTTS())
	}
	return build(ctx, cfg)
}

// ProvidersSTT ro'yxatdan o'tgan STT provayderlari nomlarini alifbo tartibida qaytaradi.
func ProvidersSTT() []string {
	names := make([]string, 0, len(sttRegistry))
	for name := range sttRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ProvidersTTS ro'yxatdan o'tgan TTS provayderlari nomlarini alifbo tartibida qaytaradi.
func ProvidersTTS() []string {
	names := make([]string, 0, len(ttsRegistry))
	for name := range ttsRegistry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
