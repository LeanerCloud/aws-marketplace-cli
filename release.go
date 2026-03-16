package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"gopkg.in/yaml.v2"
)

func validateReleaseParams(productName, newVersion, image, releaseNotes string) error {
	if productName == "" || newVersion == "" || image == "" || releaseNotes == "" {
		return errors.New("product, version, image, and release notes are all required")
	}
	return nil
}

func resolveBaseVersion(details *EntityDetails, baseVersion string) (string, error) {
	if baseVersion != "" {
		return baseVersion, nil
	}
	bv, err := latestVersion(details)
	if err != nil {
		return "", fmt.Errorf("could not auto-detect base version: %w", err)
	}
	fmt.Printf("Auto-detected base version: %s\n", bv)
	return bv, nil
}

// writeBaseVersionYAML marshals and persists the named base version from details to disk.
func writeBaseVersionYAML(productName, baseVersion string, details *EntityDetails) error {
	for i := range details.Versions {
		if details.Versions[i].VersionTitle != baseVersion {
			continue
		}
		data, err := yaml.Marshal(details.Versions[i])
		if err != nil {
			return fmt.Errorf("failed to marshal base version: %w", err)
		}
		filePath, err := getYamlFilePath(productName, "versions", baseVersion)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filePath, data, 0o644); err != nil { //nolint:gosec // G306: 0644 is intentional — user-readable YAML version files
			return fmt.Errorf("failed to write base version YAML: %w", err)
		}
		fmt.Printf("Base version written to %s\n", filePath)
		return nil
	}
	return fmt.Errorf("base version %q not found in product versions", baseVersion)
}

func releaseVersionWithClient(svc marketplaceClient, productName, newVersion, image, releaseNotes, baseVersion string, noOp bool) error {
	if err := validateReleaseParams(productName, newVersion, image, releaseNotes); err != nil {
		return err
	}

	entityID, _, err := findProduct(svc, productName)
	if err != nil {
		return err
	}

	details, err := describeProduct(svc, entityID)
	if err != nil {
		return err
	}

	baseVersion, err = resolveBaseVersion(details, baseVersion)
	if err != nil {
		return err
	}

	if err := writeBaseVersionYAML(productName, baseVersion, details); err != nil {
		return err
	}

	if err := cloneProductVersion(productName, baseVersion, newVersion); err != nil {
		return fmt.Errorf("failed to clone version: %w", err)
	}

	if err := updateVersionYAML(productName, newVersion, image, releaseNotes); err != nil {
		return fmt.Errorf("failed to update version YAML: %w", err)
	}

	return pushNewVersionWithClient(svc, productName, noOp, newVersion)
}

func releaseVersion(productName, newVersion, image, releaseNotes, baseVersion string, noOp bool) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return fmt.Errorf("couldn't load AWS config: %w", err)
	}
	return releaseVersionWithClient(marketplacecatalog.NewFromConfig(cfg), productName, newVersion, image, releaseNotes, baseVersion, noOp)
}

func updateVersionYAML(productName, version, image, releaseNotes string) error {
	filePath, err := getYamlFilePath(productName, "versions", version)
	if err != nil {
		return err
	}

	data, err := getYAMLData(filePath)
	if err != nil {
		return fmt.Errorf("failed to read version YAML: %w", err)
	}

	data.Releasenotes = releaseNotes
	data.Versiontitle = version

	for i := range data.Sources {
		for j := range data.Sources[i].Images {
			data.Sources[i].Images[j] = image
		}
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal updated YAML: %w", err)
	}

	if err := os.WriteFile(filePath, yamlBytes, 0o644); err != nil { //nolint:gosec // G306: 0644 is intentional — user-readable YAML version files
		return fmt.Errorf("failed to write updated YAML: %w", err)
	}

	fmt.Printf("Updated version YAML at %s\n", filePath)
	return nil
}
