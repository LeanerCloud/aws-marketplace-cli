package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog/types"
	"gopkg.in/yaml.v2"
)

// --- shared test helpers ---

func makeListOutput(name, entityID string) *marketplacecatalog.ListEntitiesOutput {
	return &marketplacecatalog.ListEntitiesOutput{
		EntitySummaryList: []types.EntitySummary{
			{Name: aws.String(name), EntityId: aws.String(entityID)},
		},
	}
}

func makeDescribeOutput(t *testing.T, details *EntityDetails) *marketplacecatalog.DescribeEntityOutput {
	t.Helper()
	b, err := json.Marshal(details)
	if err != nil {
		t.Fatalf("json.Marshal details: %v", err)
	}
	s := string(b)
	return &marketplacecatalog.DescribeEntityOutput{
		Details:    &s,
		EntityType: aws.String("ContainerProduct@1.0"),
	}
}

// foundMock returns a mock that finds productName under productType and describes it with details.
func foundMock(productName, entityID, productType string, details *EntityDetails, t *testing.T) *mockMarketplaceClient {
	return &mockMarketplaceClient{
		listEntitiesFunc: func(_ context.Context, params *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
			if *params.EntityType == productType {
				return makeListOutput(productName, entityID), nil
			}
			return &marketplacecatalog.ListEntitiesOutput{}, nil
		},
		describeEntityFunc: func(_ context.Context, _ *marketplacecatalog.DescribeEntityInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error) {
			return makeDescribeOutput(t, details), nil
		},
	}
}

// makeEntityDetailsWithVersion builds an EntityDetails containing a single version via JSON.
func makeEntityDetailsWithVersion(t *testing.T, versionTitle string) *EntityDetails {
	t.Helper()
	jsonStr := fmt.Sprintf(
		`{"Versions":[{"VersionTitle":%q,"ReleaseNotes":"notes","Id":"vid-1","CreationDate":"2024-01-01T00:00:00Z","Sources":[],"DeliveryOptions":[]}]}`,
		versionTitle,
	)
	var d EntityDetails
	if err := json.Unmarshal([]byte(jsonStr), &d); err != nil {
		t.Fatalf("makeEntityDetailsWithVersion: %v", err)
	}
	return &d
}

// --- tests ---

func TestGetEntityTypeAndChangeType(t *testing.T) {
	tests := []struct {
		productType    string
		wantIdentifier string
	}{
		{"ContainerProduct", "ContainerProduct@1.0"},
		{"ServerProduct", "ServerProduct@1.0"},
		{"SaaSProduct", "SaaSProduct@1.0"},
		{"UnknownType", "UnknownType@1.0"},
	}
	for _, tc := range tests {
		t.Run(tc.productType, func(t *testing.T) {
			id, changeType := getEntityTypeAndChangeType(tc.productType)
			if id != tc.wantIdentifier {
				t.Errorf("identifier = %q, want %q", id, tc.wantIdentifier)
			}
			if changeType != "UpdateInformation" {
				t.Errorf("changeType = %q, want UpdateInformation", changeType)
			}
		})
	}
}

func TestResolveProductTypes(t *testing.T) {
	tests := []struct {
		input   string
		wantLen int
		wantErr bool
	}{
		{"all", len(allProductTypes), false},
		{"ContainerProduct", 1, false},
		{"ServerProduct", 1, false},
		{"InvalidType", 0, true},
		{"", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := resolveProductTypes(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantLen {
				t.Errorf("len = %d, want %d; got %v", len(got), tc.wantLen, got)
			}
		})
	}
}

func TestPaginateEntityNames(t *testing.T) {
	t.Run("single page", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{
					EntitySummaryList: []types.EntitySummary{
						{Name: aws.String("ProductA")},
						{Name: aws.String("ProductB")},
					},
				}, nil
			},
		}
		names, err := paginateEntityNames(context.Background(), svc, &marketplacecatalog.ListEntitiesInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(names) != 2 {
			t.Errorf("len = %d, want 2", len(names))
		}
	})

	t.Run("paginated two pages", func(t *testing.T) {
		call := 0
		token := "next-token"
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				call++
				if call == 1 {
					return &marketplacecatalog.ListEntitiesOutput{
						EntitySummaryList: []types.EntitySummary{{Name: aws.String("A")}},
						NextToken:         &token,
					}, nil
				}
				return &marketplacecatalog.ListEntitiesOutput{
					EntitySummaryList: []types.EntitySummary{{Name: aws.String("B")}},
				}, nil
			},
		}
		names, err := paginateEntityNames(context.Background(), svc, &marketplacecatalog.ListEntitiesInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(names) != 2 {
			t.Errorf("len = %d, want 2; got %v", len(names), names)
		}
	})

	t.Run("first call error", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return nil, errors.New("api error")
			},
		}
		_, err := paginateEntityNames(context.Background(), svc, &marketplacecatalog.ListEntitiesInput{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("second page error", func(t *testing.T) {
		call := 0
		tok := "t"
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				call++
				if call == 1 {
					return &marketplacecatalog.ListEntitiesOutput{NextToken: &tok}, nil
				}
				return nil, errors.New("page 2 error")
			},
		}
		_, err := paginateEntityNames(context.Background(), svc, &marketplacecatalog.ListEntitiesInput{})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCollectProductNames(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{
					EntitySummaryList: []types.EntitySummary{{Name: aws.String("Prod1")}},
				}, nil
			},
		}
		names, err := collectProductNames(context.Background(), svc, "ContainerProduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(names) != 1 || names[0] != "Prod1" {
			t.Errorf("got %v", names)
		}
	})

	t.Run("validation exception returns nil", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return nil, &types.ValidationException{Message: aws.String("The entity type is invalid")}
			},
		}
		names, err := collectProductNames(context.Background(), svc, "ContainerProduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if names != nil {
			t.Errorf("expected nil names, got %v", names)
		}
	})

	t.Run("other error wrapped with product type", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return nil, errors.New("network failure")
			},
		}
		_, err := collectProductNames(context.Background(), svc, "ContainerProduct")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "ContainerProduct") {
			t.Errorf("error %q does not mention product type", err.Error())
		}
	})
}

func TestListProductsWithClient(t *testing.T) {
	t.Run("invalid type returns error", func(t *testing.T) {
		svc := &mockMarketplaceClient{}
		err := listProductsWithClient(svc, "InvalidType")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no products found", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
		}
		if err := listProductsWithClient(svc, "ContainerProduct"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("products found", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{
					EntitySummaryList: []types.EntitySummary{{Name: aws.String("MyProduct")}},
				}, nil
			},
		}
		if err := listProductsWithClient(svc, "ContainerProduct"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("list error propagated", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return nil, errors.New("list failed")
			},
		}
		if err := listProductsWithClient(svc, "ContainerProduct"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGetProductEntityID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return makeListOutput("MyProduct", "eid-123"), nil
			},
		}
		name := "MyProduct"
		id, err := getProductEntityID(svc, &name, "ContainerProduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if *id != "eid-123" {
			t.Errorf("id = %q, want eid-123", *id)
		}
	})

	t.Run("empty list returns error", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
		}
		name := "X"
		if _, err := getProductEntityID(svc, &name, "ContainerProduct"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("name not in list", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return makeListOutput("OtherProduct", "eid-999"), nil
			},
		}
		name := "MyProduct"
		if _, err := getProductEntityID(svc, &name, "ContainerProduct"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("api error", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return nil, errors.New("api error")
			},
		}
		name := "X"
		if _, err := getProductEntityID(svc, &name, "ContainerProduct"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestFindProduct(t *testing.T) {
	t.Run("found as ContainerProduct", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, params *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				if *params.EntityType == "ContainerProduct" {
					return makeListOutput("MyProduct", "eid-42"), nil
				}
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
		}
		eid, pt, err := findProduct(svc, "MyProduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if eid != "eid-42" {
			t.Errorf("entityID = %q", eid)
		}
		if pt != "ContainerProduct" {
			t.Errorf("productType = %q", pt)
		}
	})

	t.Run("not found in any type", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
		}
		_, _, err := findProduct(svc, "NonExistent")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "NonExistent") {
			t.Errorf("error %q does not mention product name", err.Error())
		}
	})
}

func TestDescribeProduct(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			describeEntityFunc: func(_ context.Context, _ *marketplacecatalog.DescribeEntityInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error) {
				return makeDescribeOutput(t, &EntityDetails{}), nil
			},
		}
		got, err := describeProduct(svc, "eid-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil result")
		}
	})

	t.Run("api error", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			describeEntityFunc: func(_ context.Context, _ *marketplacecatalog.DescribeEntityInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error) {
				return nil, errors.New("describe failed")
			},
		}
		if _, err := describeProduct(svc, "eid-1"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			describeEntityFunc: func(_ context.Context, _ *marketplacecatalog.DescribeEntityInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error) {
				bad := "not-json"
				return &marketplacecatalog.DescribeEntityOutput{Details: &bad}, nil
			},
		}
		if _, err := describeProduct(svc, "eid-1"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestGetYamlFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	path, err := getYamlFilePath("MyProduct", "versions", "v1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("data", "MyProduct", "versions", "v1.0.yaml")
	if path != expected {
		t.Errorf("path = %q, want %q", path, expected)
	}
	if _, err := os.Stat(filepath.Join("data", "MyProduct", "versions")); err != nil {
		t.Errorf("directory not created: %v", err)
	}
}

func TestWriteFileIfChanged(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.yaml")

	t.Run("new file written", func(t *testing.T) {
		if err := writeFileIfChanged(filePath, []byte("content"), "unchanged", "written"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := os.ReadFile(filePath)
		if string(got) != "content" {
			t.Errorf("file content = %q", got)
		}
	})

	t.Run("unchanged file not rewritten", func(t *testing.T) {
		if err := writeFileIfChanged(filePath, []byte("content"), "unchanged", "written"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("changed file updated", func(t *testing.T) {
		if err := writeFileIfChanged(filePath, []byte("new content"), "unchanged", "written"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, _ := os.ReadFile(filePath)
		if string(got) != "new content" {
			t.Errorf("file content = %q", got)
		}
	})
}

func TestGetYamlFilePathError(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	// Create a plain file named "data" so MkdirAll("data/...") fails.
	if err := os.WriteFile("data", []byte("not a dir"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := getYamlFilePath("MyProduct", "versions", "v1.0")
	if err == nil {
		t.Fatal("expected error when data/ is a file")
	}
}

func TestListProductsWithClientAllNoProducts(t *testing.T) {
	svc := &mockMarketplaceClient{
		listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
			return &marketplacecatalog.ListEntitiesOutput{}, nil
		},
	}
	// "all" with no products should call printNotFound("all")
	if err := listProductsWithClient(svc, "all"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDumpProductWithClient(t *testing.T) {
	t.Run("success creates description yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		svc := foundMock("MyProduct", "eid-1", "ContainerProduct", &EntityDetails{}, t)
		if err := dumpProductWithClient(svc, "MyProduct"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(filepath.Join("data", "MyProduct", "description.yaml")); err != nil {
			t.Errorf("description.yaml not created: %v", err)
		}
	})

	t.Run("findProduct error propagated", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, _ *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
		}
		if err := dumpProductWithClient(svc, "NonExistent"); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("describeProduct error propagated", func(t *testing.T) {
		svc := &mockMarketplaceClient{
			listEntitiesFunc: func(_ context.Context, params *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
				if *params.EntityType == "ContainerProduct" {
					return makeListOutput("MyProduct", "eid-1"), nil
				}
				return &marketplacecatalog.ListEntitiesOutput{}, nil
			},
			describeEntityFunc: func(_ context.Context, _ *marketplacecatalog.DescribeEntityInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error) {
				return nil, errors.New("describe failed")
			},
		}
		if err := dumpProductWithClient(svc, "MyProduct"); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUpdateProductWithClient(t *testing.T) {
	listFuncFoundAs := func(pt string) func(context.Context, *marketplacecatalog.ListEntitiesInput, ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
		return func(_ context.Context, params *marketplacecatalog.ListEntitiesInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error) {
			if *params.EntityType == pt {
				return makeListOutput("MyProduct", "eid-1"), nil
			}
			return &marketplacecatalog.ListEntitiesOutput{}, nil
		}
	}

	setupDescriptionFile := func(t *testing.T) {
		t.Helper()
		dir := filepath.Join("data", "MyProduct")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		data, _ := yaml.Marshal(EntityDetails{})
		if err := os.WriteFile(filepath.Join(dir, "description.yaml"), data, 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	t.Run("noOp prints changeset", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		setupDescriptionFile(t)
		svc := &mockMarketplaceClient{listEntitiesFunc: listFuncFoundAs("ContainerProduct")}
		if err := updateProductWithClient(svc, "MyProduct", true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("calls StartChangeSet when not noOp", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		setupDescriptionFile(t)
		called := false
		svc := &mockMarketplaceClient{
			listEntitiesFunc: listFuncFoundAs("ContainerProduct"),
			startChangeSetFunc: func(_ context.Context, _ *marketplacecatalog.StartChangeSetInput, _ ...func(*marketplacecatalog.Options)) (*marketplacecatalog.StartChangeSetOutput, error) {
				called = true
				return &marketplacecatalog.StartChangeSetOutput{}, nil
			},
		}
		if err := updateProductWithClient(svc, "MyProduct", false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Error("StartChangeSet was not called")
		}
	})

	t.Run("missing description file returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		origDir, _ := os.Getwd()
		_ = os.Chdir(tmpDir)
		defer func() { _ = os.Chdir(origDir) }()

		svc := &mockMarketplaceClient{listEntitiesFunc: listFuncFoundAs("ContainerProduct")}
		if err := updateProductWithClient(svc, "MyProduct", false); err == nil {
			t.Fatal("expected error")
		}
	})
}
