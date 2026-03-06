package memorysvc

import (
	"encoding/json"
	"net/http"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

type Config struct {
	DatabaseURL string
	RedisURL    string
}

type Service struct {
	config Config
	logger *runtimeserver.JSONLogger
}

func NewService(config Config, logger *runtimeserver.JSONLogger) *Service {
	return &Service{config: config, logger: logger}
}

func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/memory/documents", s.handleStoreDocument)
	mux.HandleFunc("GET /api/v1/memory/documents", s.handleListDocuments)
	mux.HandleFunc("GET /api/v1/memory/documents/{id}", s.handleGetDocument)
	mux.HandleFunc("DELETE /api/v1/memory/documents/{id}", s.handleDeleteDocument)

	mux.HandleFunc("POST /api/v1/memory/search", s.handleSemanticSearch)
	mux.HandleFunc("POST /api/v1/memory/recall", s.handleRecall)

	mux.HandleFunc("POST /api/v1/memory/summarize", s.handleSummarize)
	mux.HandleFunc("GET /api/v1/memory/conversations", s.handleListConversationMemories)
	mux.HandleFunc("GET /api/v1/memory/conversations/{id}", s.handleGetConversationMemory)

	mux.HandleFunc("POST /api/v1/memory/facts", s.handleStoreFact)
	mux.HandleFunc("GET /api/v1/memory/facts", s.handleListFacts)
	mux.HandleFunc("DELETE /api/v1/memory/facts/{id}", s.handleDeleteFact)

	mux.HandleFunc("POST /api/v1/memory/knowledge-graph/triples", s.handleAddTriple)
	mux.HandleFunc("GET /api/v1/memory/knowledge-graph/query", s.handleQueryGraph)
	mux.HandleFunc("GET /api/v1/memory/knowledge-graph/entity/{entity}", s.handleGetEntity)

	mux.HandleFunc("POST /api/v1/memory/forget", s.handleForget)
}

func (s *Service) handleStoreDocument(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "stored"})
}

func (s *Service) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"documents": []any{}})
}

func (s *Service) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id")})
}

func (s *Service) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleSemanticSearch(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"results": []any{}})
}

func (s *Service) handleRecall(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"memories": []any{}})
}

func (s *Service) handleSummarize(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusAccepted, map[string]any{"status": "summarizing"})
}

func (s *Service) handleListConversationMemories(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"conversations": []any{}})
}

func (s *Service) handleGetConversationMemory(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"id": r.PathValue("id")})
}

func (s *Service) handleStoreFact(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "stored"})
}

func (s *Service) handleListFacts(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"facts": []any{}})
}

func (s *Service) handleDeleteFact(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) handleAddTriple(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusCreated, map[string]any{"status": "added"})
}

func (s *Service) handleQueryGraph(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"triples": []any{}})
}

func (s *Service) handleGetEntity(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"entity": r.PathValue("entity"), "relations": []any{}})
}

func (s *Service) handleForget(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]any{"status": "forgotten"})
}

func (s *Service) respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
