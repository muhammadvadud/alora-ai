package voice

import (
	"context"
	"testing"
)

func resetRegistries() {
	sttRegistry = map[string]STTBuilder{}
	ttsRegistry = map[string]TTSBuilder{}
}

func TestSTTRegistry(t *testing.T) {
	resetRegistries()
	defer resetRegistries()

	b := func(ctx context.Context, cfg STTConfig) (STT, error) {
		return nil, nil
	}

	RegisterSTT("test_stt", b)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("ikkinchi marta ro'yxatdan o'tkazishda vahima (panic) bo'lishi kerak edi")
		}
	}()
	RegisterSTT("test_stt", b)
}

func TestNewSTT(t *testing.T) {
	resetRegistries()
	defer resetRegistries()

	_, err := NewSTT(context.Background(), STTConfig{Provider: "unknown"})
	if err == nil {
		t.Errorf("noma'lum provider uchun xato qaytishi kerak edi")
	}

	b := func(ctx context.Context, cfg STTConfig) (STT, error) {
		return nil, nil
	}
	RegisterSTT("test_stt", b)
	_, err = NewSTT(context.Background(), STTConfig{Provider: "test_stt"})
	if err != nil {
		t.Errorf("ma'lum provider yaratilishida xato kutilmagan edi: %v", err)
	}
}

func TestTTSRegistry(t *testing.T) {
	resetRegistries()
	defer resetRegistries()

	b := func(ctx context.Context, cfg TTSConfig) (TTS, error) {
		return nil, nil
	}

	RegisterTTS("test_tts", b)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("ikkinchi marta ro'yxatdan o'tkazishda vahima (panic) bo'lishi kerak edi")
		}
	}()
	RegisterTTS("test_tts", b)
}

func TestNewTTS(t *testing.T) {
	resetRegistries()
	defer resetRegistries()

	_, err := NewTTS(context.Background(), TTSConfig{Provider: "unknown"})
	if err == nil {
		t.Errorf("noma'lum provider uchun xato qaytishi kerak edi")
	}

	b := func(ctx context.Context, cfg TTSConfig) (TTS, error) {
		return nil, nil
	}
	RegisterTTS("test_tts", b)
	_, err = NewTTS(context.Background(), TTSConfig{Provider: "test_tts"})
	if err != nil {
		t.Errorf("ma'lum provider yaratilishida xato kutilmagan edi: %v", err)
	}
}

func TestProviders(t *testing.T) {
	resetRegistries()
	defer resetRegistries()

	RegisterSTT("stt_1", nil)
	RegisterSTT("stt_2", nil)

	sttNames := ProvidersSTT()
	if len(sttNames) != 2 || sttNames[0] != "stt_1" || sttNames[1] != "stt_2" {
		t.Errorf("STT provayderlari ro'yxati xato: %v", sttNames)
	}

	RegisterTTS("tts_2", nil)
	RegisterTTS("tts_1", nil)

	ttsNames := ProvidersTTS()
	if len(ttsNames) != 2 || ttsNames[0] != "tts_1" || ttsNames[1] != "tts_2" {
		t.Errorf("TTS provayderlari ro'yxati xato: %v", ttsNames)
	}
}
