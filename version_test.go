package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"gopkg.in/yaml.v2"
)

func TestConvertDeliveryOption(t *testing.T) {
	opt := Deliveryoptions{
		Title:            "Helm Chart",
		Shortdescription: "Deploy via Helm",
		Instructions:     Instructions{Usage: "helm install ..."},
		Compatibility:    ServicesCompatibility{Awsservices: []string{"EKS"}},
		Recommendations: Recommendations{
			Deploymentresources: []Deploymentresources{
				{Text: "Docs", URL: "https://example.com"},
			},
		},
	}
	got := convertDeliveryOption(opt, []string{"my-ecr:latest"})

	if got.DeliveryOptionTitle != "Helm Chart" {
		t.Errorf("title = %q", got.DeliveryOptionTitle)
	}
	ecr := got.Details.EcrDeliveryOptionDetails
	if ecr.Description != "Deploy via Helm" {
		t.Errorf("description = %q", ecr.Description)
	}
	if ecr.UsageInstructions != "helm install ..." {
		t.Errorf("usage = %q", ecr.UsageInstructions)
	}
	if len(ecr.ContainerImages) != 1 || ecr.ContainerImages[0] != "my-ecr:latest" {
		t.Errorf("images = %v", ecr.ContainerImages)
	}
	if len(ecr.CompatibleServices) != 1 || ecr.CompatibleServices[0] != "EKS" {
		t.Errorf("services = %v", ecr.CompatibleServices)
	}
	if len(ecr.DeploymentResources) != 1 {
		t.Fatalf("resources len = %d", len(ecr.DeploymentResources))
	}
	if ecr.DeploymentResources[0].Name != "Docs" || ecr.DeploymentResources[0].URL != "https://example.com" {
		t.Errorf("resource = %+v", ecr.DeploymentResources[0])
	}
}

func TestConvertToDst(t *testing.T) {
	t.Run("with sources and delivery options", func(t *testing.T) {
		src := YAMLVersionData{
			Releasenotes: "My notes",
			Versiontitle: "v2.0",
			Sources:      []Sources{{Images: []string{"ecr:v2"}}},
			Deliveryoptions: []Deliveryoptions{
				{Title: "Option A"},
				{Title: "Option B"},
			},
		}
		dst := src.convertToDst()
		if dst.Version.ReleaseNotes != "My notes" {
			t.Errorf("release notes = %q", dst.Version.ReleaseNotes)
		}
		if dst.Version.VersionTitle != "v2.0" {
			t.Errorf("version title = %q", dst.Version.VersionTitle)
		}
		if len(dst.DeliveryOptions) != 2 {
			t.Errorf("delivery options = %d, want 2", len(dst.DeliveryOptions))
		}
		for _, do := range dst.DeliveryOptions {
			imgs := do.Details.EcrDeliveryOptionDetails.ContainerImages
			if len(imgs) != 1 || imgs[0] != "ecr:v2" {
				t.Errorf("delivery option images = %v", imgs)
			}
		}
	})

	t.Run("no sources produces empty images", func(t *testing.T) {
		src := YAMLVersionData{
			Releasenotes:    "notes",
			Versiontitle:    "v1.0",
			Deliveryoptions: []Deliveryoptions{{Title: "Opt"}},
		}
		dst := src.convertToDst()
		if len(dst.DeliveryOptions) != 1 {
			t.Fatalf("delivery options = %d", len(dst.DeliveryOptions))
		}
		if len(dst.DeliveryOptions[0].Details.EcrDeliveryOptionDetails.ContainerImages) != 0 {
			t.Error("expected no container images when no sources")
		}
	})
}

func TestGetYAMLData(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		tmpDir := t.TempDir()
		data := YAMLVersionData{Versiontitle: "v1.0", Releasenotes: "test notes"}
		b, _ := yaml.Marshal(data)
		filePath := filepath.Join(tmpDir, "test.yaml")
		_ = os.WriteFile(filePath, b, 0o644)

		got, err := getYAMLData(filePath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Versiontitle != "v1.0" {
			t.Errorf("version = %q", got.Versiontitle)
		}
		if got.Releasenotes != "test notes" {
			t.Errorf("notes = %q", got.Releasenotes)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		if _, err := getYAMLData("/nonexistent/path/file.yaml"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCloneProductVersion(t *testing.T) {
	t.Run("creates dst from src with version replacement", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		dir := filepath.Join("data", "MyProduct", "versions")
		_ = os.MkdirAll(dir, 0o755)
		src := YAMLVersionData{Versiontitle: "v1.0", Releasenotes: "initial"}
		b, _ := yaml.Marshal(src)
		_ = os.WriteFile(filepath.Join(dir, "v1.0.yaml"), b, 0o644)

		if err := cloneProductVersion("MyProduct", "v1.0", "v2.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		dstBytes, err := os.ReadFile(filepath.Join(dir, "v2.0.yaml"))
		if err != nil {
			t.Fatalf("dst file not created: %v", err)
		}
		if !strings.Contains(string(dstBytes), "v2.0") {
			t.Errorf("dst file doesn't contain new version: %s", dstBytes)
		}
	})

	t.Run("unchanged dst not overwritten", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		dir := filepath.Join("data", "MyProduct", "versions")
		_ = os.MkdirAll(dir, 0o755)
		content := []byte("same content")
		_ = os.WriteFile(filepath.Join(dir, "v1.0.yaml"), content, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "v2.0.yaml"), content, 0o644)

		if err := cloneProductVersion("MyProduct", "v1.0", "v2.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("source file not found returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		if err := cloneProductVersion("MyProduct", "nonexistent", "v2.0"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDumpVersionsWithClient(t *testing.T) {
	t.Run("success writes version yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		details := makeEntityDetailsWithVersion(t, "v1.0")
		svc := foundMock(t, "MyProduct", "eid-1", productTypeContainer, details)

		if err := dumpVersionsWithClient(svc, "MyProduct"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(filepath.Join("data", "MyProduct", "versions", "v1.0.yaml")); err != nil {
			t.Errorf("version file not created: %v", err)
		}
	})

	t.Run("findProduct error propagated", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
		}
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		if err := dumpVersionsWithClient(svc, "NonExistent"); err == nil {
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
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		if err := dumpVersionsWithClient(svc, "MyProduct"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPushNewVersionWithClient(t *testing.T) {
	listFoundAs := func(pt string) func(context.Context, *marketplacecatalog.ListEntitiesInput, ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
		return func(_ context.Context, params *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
			if *params.EntityType == pt {
				return makeListOutput("MyProduct", "eid-1"), nil
			}
			return &marketplacecatalog.ListEntitiesOutput{}, nil
		}
	}

	writeVersionFile := func(t *testing.T, version string, data YAMLVersionData) {
		t.Helper()
		dir := filepath.Join("data", "MyProduct", "versions")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		b, _ := yaml.Marshal(data)
		if err := os.WriteFile(filepath.Join(dir, version+".yaml"), b, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	t.Run("noOp prints changeset without calling StartChangeSet", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		writeVersionFile(t, "v1.0", YAMLVersionData{
			Versiontitle:    "v1.0",
			Releasenotes:    "notes",
			Sources:         []Sources{{Images: []string{"ecr:v1"}}},
			Deliveryoptions: []Deliveryoptions{{Title: "Option A"}},
		})
		svc := &mockMarketplaceClient{listEntitiesFunc: listFoundAs(productTypeContainer)}
		if err := pushNewVersionWithClient(svc, "MyProduct", true, "v1.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("calls StartChangeSet for ContainerProduct with AddDeliveryOptions", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		writeVersionFile(t, "v1.0", YAMLVersionData{Versiontitle: "v1.0"})
		var gotChangeType string
		svc := &mockMarketplaceClient{
			listEntitiesFunc: listFoundAs(productTypeContainer),
			startChangeSetFunc: func(_ context.Context, params *marketplacecatalog.StartChangeSetInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.StartChangeSetOutput, error) {
				gotChangeType = *params.ChangeSet[0].ChangeType
				return &marketplacecatalog.StartChangeSetOutput{}, nil
			},
		}
		if err := pushNewVersionWithClient(svc, "MyProduct", false, "v1.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotChangeType != "AddDeliveryOptions" {
			t.Errorf("changeType = %q, want AddDeliveryOptions", gotChangeType)
		}
	})

	t.Run("uses CreateVersion for ServerProduct", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		writeVersionFile(t, "v1.0", YAMLVersionData{Versiontitle: "v1.0"})
		var gotChangeType string
		svc := &mockMarketplaceClient{
			listEntitiesFunc: listFoundAs(productTypeServer),
			startChangeSetFunc: func(_ context.Context, params *marketplacecatalog.StartChangeSetInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.StartChangeSetOutput, error) {
				gotChangeType = *params.ChangeSet[0].ChangeType
				return &marketplacecatalog.StartChangeSetOutput{}, nil
			},
		}
		if err := pushNewVersionWithClient(svc, "MyProduct", false, "v1.0"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotChangeType != "CreateVersion" {
			t.Errorf("changeType = %q, want CreateVersion", gotChangeType)
		}
	})

	t.Run("missing version file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		svc := &mockMarketplaceClient{listEntitiesFunc: listFoundAs(productTypeContainer)}
		err := pushNewVersionWithClient(svc, "MyProduct", false, "nonexistent")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "could not read version details") {
			t.Errorf("error = %q", err.Error())
		}
	})

	t.Run("StartChangeSet error returned", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		writeVersionFile(t, "v1.0", YAMLVersionData{Versiontitle: "v1.0"})
		svc := &mockMarketplaceClient{
			listEntitiesFunc: listFoundAs(productTypeContainer),
			startChangeSetFunc: func(_ context.Context, _ *marketplacecatalog.StartChangeSetInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.StartChangeSetOutput, error) {
				return nil, errors.New("change set failed")
			},
		}
		err := pushNewVersionWithClient(svc, "MyProduct", false, "v1.0")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "could not start change set") {
			t.Errorf("error = %q", err.Error())
		}
	})
}

