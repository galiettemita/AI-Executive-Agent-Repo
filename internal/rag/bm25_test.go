package rag_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/brevio/brevio/internal/rag"
)

func TestBM25_ExactMatch_BeatsPartial(t *testing.T) {
	idx := rag.NewBM25Index()
	full := rag.BM25Tokenize("anthropic series b funding round announcement")
	partial := rag.BM25Tokenize("anthropic announcement")
	idx.IndexDocument("full-doc", full)
	idx.IndexDocument("partial-doc", partial)

	query := rag.BM25Tokenize("anthropic series b funding")
	scoreFullDoc := idx.Score(full, query)
	scorePartialDoc := idx.Score(partial, query)
	if scoreFullDoc <= scorePartialDoc {
		t.Fatalf("full match (%v) should score > partial match (%v)", scoreFullDoc, scorePartialDoc)
	}
}

func TestBM25_IDFWeighting_RareTermScoresHigher(t *testing.T) {
	idx := rag.NewBM25Index()
	for i := 0; i < 9; i++ {
		idx.IndexDocument(fmt.Sprintf("doc-%d", i), rag.BM25Tokenize("meeting today schedule"))
	}
	idx.IndexDocument("doc-9", rag.BM25Tokenize("anthropic series b funding"))

	targetDoc := rag.BM25Tokenize("anthropic series b funding")
	rareScore := idx.Score(targetDoc, rag.BM25Tokenize("anthropic"))
	commonScore := idx.Score(targetDoc, rag.BM25Tokenize("meeting"))
	if rareScore <= commonScore {
		t.Fatalf("rare term score (%v) should > common term score (%v)", rareScore, commonScore)
	}
}

func TestBM25_ZeroScore_NoOverlap(t *testing.T) {
	idx := rag.NewBM25Index()
	idx.IndexDocument("doc-1", rag.BM25Tokenize("the quick brown fox"))
	score := idx.Score(rag.BM25Tokenize("the quick brown fox"), rag.BM25Tokenize("anthropic series"))
	if score != 0.0 {
		t.Fatalf("expected 0.0 for no term overlap, got %v", score)
	}
}

func TestBM25_EmptyCorpus_NoPanic(t *testing.T) {
	idx := rag.NewBM25Index()
	score := idx.Score(rag.BM25Tokenize("some text"), rag.BM25Tokenize("some query"))
	if score != 0.0 {
		t.Fatalf("expected 0.0 for empty corpus, got %v", score)
	}
}

func TestBM25_StopwordsFiltered(t *testing.T) {
	withStop := rag.BM25Tokenize("the meeting at the office")
	withoutStop := rag.BM25Tokenize("meeting office")
	if len(withStop) != len(withoutStop) {
		t.Logf("withStop=%v withoutStop=%v", withStop, withoutStop)
		t.Fatal("stopwords not filtered — token counts differ")
	}
}

func TestBM25_Tokenize_HyphenatedTerms(t *testing.T) {
	tokens := rag.BM25Tokenize("Q3-budget series-b")
	if len(tokens) == 0 {
		t.Fatal("expected non-empty token list for hyphenated terms")
	}
}

func TestBM25_DocumentLengthNorm(t *testing.T) {
	idx := rag.NewBM25Index()
	shortDoc := rag.BM25Tokenize("anthropic")
	longDoc := rag.BM25Tokenize("anthropic " + strings.Repeat("filler word token padding extra text long document length normalization ", 20))
	idx.IndexDocument("short", shortDoc)
	idx.IndexDocument("long", longDoc)

	query := rag.BM25Tokenize("anthropic")
	scoreShort := idx.Score(shortDoc, query)
	scoreLong := idx.Score(longDoc, query)
	if scoreShort < scoreLong {
		t.Fatalf("short doc (%v) should score >= long doc (%v) for same single term", scoreShort, scoreLong)
	}
}

func TestBM25_Stats(t *testing.T) {
	idx := rag.NewBM25Index()
	idx.IndexDocument("d1", rag.BM25Tokenize("hello world"))
	idx.IndexDocument("d2", rag.BM25Tokenize("hello foo bar"))

	docs, vocab, avgLen := idx.Stats()
	if docs != 2 {
		t.Fatalf("docs=%d, want 2", docs)
	}
	if vocab < 3 {
		t.Fatalf("vocab=%d, want >= 3", vocab)
	}
	if math.Abs(avgLen-2.5) > 0.5 {
		t.Fatalf("avgLen=%v, want ~2.5", avgLen)
	}
}
