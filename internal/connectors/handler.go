package connectors

import (
	"encoding/json"
	"net/http"
)

// ToolRegistryResponse is the JSON envelope for GET /v1/tools.
type ToolRegistryResponse struct {
	ConnectorCount int             `json:"connector_count"`
	ToolCount      int             `json:"tool_count"`
	Connectors     []Connector     `json:"connectors"`
	Tools          []ConnectorTool `json:"tools"`
}

// RegisterRoutes registers the tool registry HTTP endpoints on the given mux.
func (s *Service) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/tools", s.handleListTools)
}

func (s *Service) handleListTools(w http.ResponseWriter, r *http.Request) {
	connectorList := s.ListConnectors()
	toolList := s.ListAllTools()

	resp := ToolRegistryResponse{
		ConnectorCount: len(connectorList),
		ToolCount:      len(toolList),
		Connectors:     connectorList,
		Tools:          toolList,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
