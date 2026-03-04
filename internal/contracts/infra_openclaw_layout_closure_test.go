package contracts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInfraTerraformModuleLayoutClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	modulesRoot := filepath.Join(root, "infra", "terraform", "modules")
	requiredModules := []string{
		"eks",
		"rds",
		"elasticache",
		"sqs-sns",
		"s3",
		"secrets",
		"cloudfront",
		"route53",
		"monitoring",
		"waf",
	}

	for _, moduleName := range requiredModules {
		moduleFile := filepath.Join(modulesRoot, moduleName, "main.tf")
		info, err := os.Stat(moduleFile)
		if err != nil {
			t.Fatalf("missing infra module file %s: %v", moduleFile, err)
		}
		if info.IsDir() {
			t.Fatalf("expected file but found directory: %s", moduleFile)
		}
		assertFileContainsTokens(t, moduleFile, []string{
			"required_version",
			"module_contract",
		})
	}

	assertFileContainsTokens(t, filepath.Join(modulesRoot, "README.md"), []string{
		"Module Map",
		"`eks`",
		"`waf`",
		"`cloudfront`",
		"`route53`",
	})
}

func TestInfraTerraformEnvironmentCompositionClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	environmentsRoot := filepath.Join(root, "infra", "terraform", "environments")
	requiredEnvs := []string{"staging", "production", "dr"}
	requiredModuleRefs := []string{
		"module \"eks\"",
		"module \"rds\"",
		"module \"elasticache\"",
		"module \"sqs_sns\"",
		"module \"s3\"",
		"module \"secrets\"",
		"module \"cloudfront\"",
		"module \"route53\"",
		"module \"monitoring\"",
		"module \"waf\"",
		"output \"environment_contract\"",
	}

	for _, envName := range requiredEnvs {
		mainFile := filepath.Join(environmentsRoot, envName, "main.tf")
		assertFileNonEmpty(t, mainFile)
		assertFileContainsTokens(t, mainFile, requiredModuleRefs)
	}
}
