package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

func TestUpdateVersionYAML(t *testing.T) {
	tests := []struct {
		name         string
		input        YAMLVersionData
		image        string
		releaseNotes string
		wantImages   []string
		wantNotes    string
		wantTitle    string
	}{
		{
			name: "replaces single image and release notes",
			input: YAMLVersionData{
				Versiontitle: "old-version",
				Releasenotes: "old notes",
				Sources: []Sources{
					{
						Type:   "DockerImages",
						Images: []string{"old-ecr-uri:old-tag"},
					},
				},
				Deliveryoptions: []Deliveryoptions{
					{Title: "delivery1"},
				},
			},
			image:        "new-ecr-uri:new-tag",
			releaseNotes: "New feature release",
			wantImages:   []string{"new-ecr-uri:new-tag"},
			wantNotes:    "New feature release",
			wantTitle:    "test-version",
		},
		{
			name: "replaces multiple images across sources",
			input: YAMLVersionData{
				Versiontitle: "old-version",
				Releasenotes: "old notes",
				Sources: []Sources{
					{
						Type:   "DockerImages",
						Images: []string{"uri1:tag1", "uri2:tag2"},
					},
					{
						Type:   "DockerImages",
						Images: []string{"uri3:tag3"},
					},
				},
			},
			image:        "new-ecr:latest",
			releaseNotes: "Multi-image update",
			wantImages:   []string{"new-ecr:latest"},
			wantNotes:    "Multi-image update",
			wantTitle:    "test-version",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp directory structure
			tmpDir := t.TempDir()
			origDir, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(origDir)

			productName := "TestProduct"
			version := "test-version"

			// Write the input YAML
			dir := filepath.Join("data", productName, "versions")
			os.MkdirAll(dir, 0755)
			inputBytes, err := yaml.Marshal(tc.input)
			if err != nil {
				t.Fatalf("failed to marshal input: %v", err)
			}
			filePath := filepath.Join(dir, version+".yaml")
			if err := os.WriteFile(filePath, inputBytes, 0644); err != nil {
				t.Fatalf("failed to write input YAML: %v", err)
			}

			// Run updateVersionYAML
			if err := updateVersionYAML(productName, version, tc.image, tc.releaseNotes); err != nil {
				t.Fatalf("updateVersionYAML() error: %v", err)
			}

			// Read back and verify
			result, err := getYAMLData(filePath)
			if err != nil {
				t.Fatalf("failed to read result: %v", err)
			}

			if result.Releasenotes != tc.wantNotes {
				t.Errorf("release notes = %q, want %q", result.Releasenotes, tc.wantNotes)
			}

			if result.Versiontitle != tc.wantTitle {
				t.Errorf("version title = %q, want %q", result.Versiontitle, tc.wantTitle)
			}

			// All images in all sources should be the new image
			for i, src := range result.Sources {
				for j, img := range src.Images {
					if img != tc.image {
						t.Errorf("source[%d].images[%d] = %q, want %q", i, j, img, tc.image)
					}
				}
			}
		})
	}
}

func TestLatestVersion(t *testing.T) {
	tests := []struct {
		name    string
		details *EntityDetails
		want    string
		wantErr bool
	}{
		{
			name:    "no versions returns error",
			details: &EntityDetails{},
			wantErr: true,
		},
		{
			name: "single version",
			details: &EntityDetails{
				Versions: []struct {
					ID                  string    `json:"Id"`
					ReleaseNotes        string    `json:"ReleaseNotes"`
					UpgradeInstructions string    `json:"UpgradeInstructions"`
					VersionTitle        string    `json:"VersionTitle"`
					CreationDate        time.Time `json:"CreationDate"`
					Sources             []struct {
						Type          string   `json:"Type"`
						ID            string   `json:"Id"`
						Images        []string `json:"Images"`
						Compatibility struct {
							Platform string `json:"Platform"`
						} `json:"Compatibility"`
					} `json:"Sources"`
					DeliveryOptions []struct {
						ID               string `json:"Id"`
						Type             string `json:"Type"`
						SourceID         string `json:"SourceId"`
						Title            string `json:"Title"`
						ShortDescription string `json:"ShortDescription"`
						IsRecommended    bool   `json:"isRecommended"`
						Compatibility    struct {
							AWSServices []string `json:"AWSServices"`
						} `json:"Compatibility"`
						Instructions struct {
							Usage string `json:"Usage"`
						} `json:"Instructions"`
						Recommendations struct {
							DeploymentResources []struct {
								Text string `json:"Text"`
								URL  string `json:"Url"`
							} `json:"DeploymentResources"`
						} `json:"Recommendations"`
						Visibility string `json:"Visibility"`
					} `json:"DeliveryOptions"`
				}{
					{VersionTitle: "v1.0", CreationDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
				},
			},
			want: "v1.0",
		},
		{
			name: "picks newest by creation date",
			details: &EntityDetails{
				Versions: []struct {
					ID                  string    `json:"Id"`
					ReleaseNotes        string    `json:"ReleaseNotes"`
					UpgradeInstructions string    `json:"UpgradeInstructions"`
					VersionTitle        string    `json:"VersionTitle"`
					CreationDate        time.Time `json:"CreationDate"`
					Sources             []struct {
						Type          string   `json:"Type"`
						ID            string   `json:"Id"`
						Images        []string `json:"Images"`
						Compatibility struct {
							Platform string `json:"Platform"`
						} `json:"Compatibility"`
					} `json:"Sources"`
					DeliveryOptions []struct {
						ID               string `json:"Id"`
						Type             string `json:"Type"`
						SourceID         string `json:"SourceId"`
						Title            string `json:"Title"`
						ShortDescription string `json:"ShortDescription"`
						IsRecommended    bool   `json:"isRecommended"`
						Compatibility    struct {
							AWSServices []string `json:"AWSServices"`
						} `json:"Compatibility"`
						Instructions struct {
							Usage string `json:"Usage"`
						} `json:"Instructions"`
						Recommendations struct {
							DeploymentResources []struct {
								Text string `json:"Text"`
								URL  string `json:"Url"`
							} `json:"DeploymentResources"`
						} `json:"Recommendations"`
						Visibility string `json:"Visibility"`
					} `json:"DeliveryOptions"`
				}{
					{VersionTitle: "v1.0", CreationDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
					{VersionTitle: "v1.2", CreationDate: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)},
					{VersionTitle: "v1.1", CreationDate: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)},
				},
			},
			want: "v1.2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := latestVersion(tc.details)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("latestVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReleaseVersionValidation(t *testing.T) {
	tests := []struct {
		name         string
		product      string
		version      string
		image        string
		releaseNotes string
		wantErr      string
	}{
		{
			name:    "missing product",
			product: "", version: "v1", image: "img:1", releaseNotes: "notes",
			wantErr: "product, version, image, and release notes are all required",
		},
		{
			name:    "missing version",
			product: "P", version: "", image: "img:1", releaseNotes: "notes",
			wantErr: "product, version, image, and release notes are all required",
		},
		{
			name:    "missing image",
			product: "P", version: "v1", image: "", releaseNotes: "notes",
			wantErr: "product, version, image, and release notes are all required",
		},
		{
			name:    "missing release notes",
			product: "P", version: "v1", image: "img:1", releaseNotes: "",
			wantErr: "product, version, image, and release notes are all required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := releaseVersion(tc.product, tc.version, tc.image, tc.releaseNotes, "", true)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tc.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tc.wantErr)
			}
		})
	}
}
