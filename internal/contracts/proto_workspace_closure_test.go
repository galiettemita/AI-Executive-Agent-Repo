package contracts

import (
	"path/filepath"
	"testing"
)

func TestProtoWorkspaceClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	protoRoot := filepath.Join(root, "packages", "proto")

	assertFileNonEmpty(t, filepath.Join(protoRoot, "package.json"))
	assertFileContainsTokens(t, filepath.Join(protoRoot, "package.json"), []string{
		"\"name\": \"@brevio/proto\"",
		"\"lint\": \"bash ./scripts/lint.sh\"",
		"\"build\": \"bash ./scripts/generate.sh\"",
	})

	assertFileContainsTokens(t, filepath.Join(protoRoot, "buf.yaml"), []string{
		"version: v2",
		"use:",
		"STANDARD",
	})
	assertFileContainsTokens(t, filepath.Join(protoRoot, "buf.gen.yaml"), []string{
		"version: v2",
		"buf.build/bufbuild/es",
		"buf.build/connectrpc/es",
		"out: gen/es",
	})

	assertFileContainsTokens(t, filepath.Join(protoRoot, "scripts", "lint.sh"), []string{
		"buf lint",
		"docker run",
		"bufbuild/buf",
	})
	assertFileContainsTokens(t, filepath.Join(protoRoot, "scripts", "generate.sh"), []string{
		"buf generate",
		"docker run",
		"bufbuild/buf",
		"mkdir -p",
	})

	assertFileContainsTokens(t, filepath.Join(root, "Makefile"), []string{
		"proto-validate:",
		"bash packages/proto/scripts/lint.sh",
		"ci: proto-validate",
	})
}
