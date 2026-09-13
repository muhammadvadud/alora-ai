// Package http agentni HTTP xizmat sifatida ochadi (JSON + SSE streaming).
package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/muhammadvadud/alora-agent/internal/agent"
)

type handler struct {
	svc *agent.Chat
}

// Serve HTTP serverini ishga tushiradi va ctx bekor bo'lganda toza to'xtatadi.
func Serve(ctx context.Context, chat *agent.Chat, addr string) error {
	h := &handler{svc: chat}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("POST /chat", h.chat)
	mux.HandleFunc("POST /chat/stream", h.chatStream)

	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("HTTP server tinglayapti", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

type chatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type toolInfo struct {
	Name   string `json:"name"`
	Args   string `json:"args"`
	Result string `json:"result,omitempty"`
}

type chatResponse struct {
	SessionID string     `json:"session_id"`
	Response  string     `json:"response"`
	Tools     []toolInfo `json:"tools,omitempty"`
}

func (h *handler) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// chat butun javobni yig'ib JSON qaytaradi (streaming emas).
func (h *handler) chat(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeRequest(w, r)
	if !ok {
		return
	}

	var (
		resp  chatResponse
		tools []toolInfo
		text  string
	)
	resp.SessionID = req.SessionID

	for ev := range h.svc.Stream(r.Context(), req.SessionID, req.Message) {
		switch ev.Kind {
		case agent.KindContent:
			text += ev.Content
		case agent.KindToolCall:
			tools = append(tools, toolInfo{Name: ev.Tool})
		case agent.KindToolArgs:
			setToolArgs(tools, ev.Tool, ev.Args)
		case agent.KindToolResult:
			setToolResult(tools, ev.Tool, ev.Result)
		case agent.KindError:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": ev.Err})
			return
		}
	}

	resp.Response = text
	resp.Tools = tools
	writeJSON(w, http.StatusOK, resp)
}

// chatStream javobni Server-Sent Events sifatida oqim qiladi.
func (h *handler) chatStream(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeRequest(w, r)
	if !ok {
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming qo'llab-quvvatlanmaydi", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	enc := json.NewEncoder(w)
	for ev := range h.svc.Stream(r.Context(), req.SessionID, req.Message) {
		if _, err := w.Write([]byte("data: ")); err != nil {
			return
		}
		if err := enc.Encode(ev); err != nil { // Encode oxiriga \n qo'shadi
			return
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return
		}
		flusher.Flush()
	}
}

// decodeRequest so'rovni o'qiydi, xabarni tekshiradi va session_id ni ta'minlaydi.
func decodeRequest(w http.ResponseWriter, r *http.Request) (chatRequest, bool) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "noto'g'ri JSON", http.StatusBadRequest)
		return req, false
	}
	if req.Message == "" {
		http.Error(w, "message bo'sh bo'lishi mumkin emas", http.StatusBadRequest)
		return req, false
	}
	if req.SessionID == "" {
		req.SessionID = newSessionID()
	}
	return req, true
}

func setToolArgs(tools []toolInfo, name, args string) {
	for i := range tools {
		if tools[i].Name == name && tools[i].Args == "" {
			tools[i].Args = args
			return
		}
	}
}

func setToolResult(tools []toolInfo, name, result string) {
	for i := range tools {
		if tools[i].Name == name && tools[i].Result == "" {
			tools[i].Result = result
			return
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
