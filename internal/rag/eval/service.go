package eval

import "sync"

type Score struct {
	CollectionID string  `json:"collection_id"`
	Faithfulness float64 `json:"faithfulness"`
	Relevance    float64 `json:"relevance"`
	Pass         bool    `json:"pass"`
}

type Service struct {
	mu     sync.RWMutex
	scores map[string]Score
}

func NewService() *Service {
	return &Service{
		scores: map[string]Score{},
	}
}

func (s *Service) Evaluate(collectionID string, faithfulness, relevance float64) Score {
	s.mu.Lock()
	defer s.mu.Unlock()
	score := Score{
		CollectionID: collectionID,
		Faithfulness: faithfulness,
		Relevance:    relevance,
		Pass:         faithfulness >= 0.80 && relevance >= 0.75,
	}
	s.scores[collectionID] = score
	return score
}

func (s *Service) Get(collectionID string) (Score, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	score, ok := s.scores[collectionID]
	return score, ok
}
