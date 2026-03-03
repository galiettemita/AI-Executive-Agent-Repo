# RAG Eval Framework

This framework defines deterministic pass/fail checks for retrieval-augmented generation in Brevio.

## Goals
- Measure **faithfulness**: generated statements must be grounded in retrieved evidence.
- Measure **relevance**: retrieved passages should match the user request intent.
- Enforce stable thresholds for CI regression gates.

## Inputs
- Query text from user intent processing.
- Retrieved chunks with source metadata (`source_id`, `chunk_id`, `score`).
- Final synthesized response text.

## Metrics
- `faithfulness` (0.0-1.0): proportion of response claims that are directly supported by retrieved chunks.
- `relevance` (0.0-1.0): average semantic relevance of retrieved chunks to the query.
- `pass`: `faithfulness >= 0.80` and `relevance >= 0.75`.

## Evaluation Procedure
1. Retrieve top-k chunks for each query.
2. Extract atomic response claims (sentence-level).
3. Attempt claim-to-chunk alignment.
4. Compute faithfulness and relevance scores.
5. Persist scores and compare against baseline drift thresholds.

## Failure Handling
- If `faithfulness < 0.80`, mark the run as failed and block promotion.
- If `relevance < 0.75`, flag retrieval quality degradation and block promotion.
- Store failing examples for root-cause analysis in eval artifacts.
