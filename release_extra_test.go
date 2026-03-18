package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
)

func TestValidateReleaseParams(t *testing.T) {
	tests := []struct {
		name, product, version, image, notes string
		wantErr                               bool
	}{
		{"all valid", "P", "v1", "img:1", "notes", false},
		{"missing product", "", "v1", "img:1", "notes", true},
		{"missing version", "P", "", "img:1", "notes", true},
		{"missing image", "P", "v1", "", "notes", true},
		{"missing notes", "P", "v1", "img:1", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateReleaseParams(tc.product, tc.version, tc.image, tc.notes)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveBaseVersion(t *testing.T) {
	t.Run("explicit base version returned as-is", func(t *testing.T) {
		got, err := resolveBaseVersion(&EntityDetails{}, "v1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v1.0" {
			t.Errorf("got %q, want v1.0", got)
		}
	})

	t.Run("auto-detects latest when empty", func(t *testing.T) {
		details := makeEntityDetailsWithVersion(t, "v2.0")
		got, err := resolveBaseVersion(details, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "v2.0" {
			t.Errorf("got %q, want v2.0", got)
		}
	})

	t.Run("no versions returns error when base is empty", func(t *testing.T) {
		_, err := resolveBaseVersion(&EntityDetails{}, "")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestWriteBaseVersionYAML(t *testing.T) {
	t.Run("writes matching version to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		details := makeEntityDetailsWithVersion(t, "v1.0")
		if err := writeBaseVersionYAML("MyProduct", "v1.0", details); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(filepath.Join("data", "MyProduct", "versions", "v1.0.yaml")); err != nil {
			t.Errorf("file not created: %v", err)
		}
	})

	t.Run("version not found returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		details := makeEntityDetailsWithVersion(t, "v1.0")
		if err := writeBaseVersionYAML("MyProduct", "nonexistent", details); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUpdateVersionYAMLMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// getYamlFilePath creates the dir but no file exists — getYAMLData should fail
	err := updateVersionYAML("MyProduct", "v1.0", "img:v1", "notes")
	if err == nil {
		t.Fatal("expected error for missing version file")
	}
}

func TestReleaseVersionWithClient(t *testing.T) {
	t.Run("noOp completes full flow without StartChangeSet", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		details := makeEntityDetailsWithVersion(t, "v1.0")
		svc := foundMock(t, "MyProduct", "eid-1", productTypeContainer, details)

		err := releaseVersionWithClient(svc, "MyProduct", "v2.0", "ecr:v2", "Release notes", "v1.0", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Both the base and new version files should exist
		if _, err := os.Stat(filepath.Join("data", "MyProduct", "versions", "v1.0.yaml")); err != nil {
			t.Errorf("base version file missing: %v", err)
		}
		if _, err := os.Stat(filepath.Join("data", "MyProduct", "versions", "v2.0.yaml")); err != nil {
			t.Errorf("new version file missing: %v", err)
		}
	})

	t.Run("validation failure returns error immediately", func(t *testing.T) {
		svc := foundMock(t, "MyProduct", "eid-1", productTypeContainer, &EntityDetails{})
		err := releaseVersionWithClient(svc, "MyProduct", "v2.0", "", "notes", "v1.0", true)
		if err == nil {
			t.Fatal("expected error for missing image")
		}
	})

	t.Run("findProduct error propagated", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
		}
		err := releaseVersionWithClient(svc, "NonExistent", "v2.0", "img:1", "notes", "v1.0", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("describeProduct error propagated", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, params *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				if *params.EntityType == productTypeContainer {
					return makeListOutput("MyProduct", "eid-1"), nil
				}
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
			describeEntityFunc: func(_ context.Context, _ *marketplacecatalog.DescribeEntityInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error) {
				return nil, errors.New("describe failed")
			},
		}
		err := releaseVersionWithClient(svc, "MyProduct", "v2.0", "img:1", "notes", "v1.0", true)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
