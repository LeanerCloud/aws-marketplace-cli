package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"gopkg.in/yaml.v2"
)

func releaseVersion(productName, newVersion, image, releaseNotes string, baseVersion string, noOp bool) error {
	if productName == "" || newVersion == "" || image == "" || releaseNotes == "" {
		return fmt.Errorf("product, version, image, and release notes are all required")
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return fmt.Errorf("couldn't load AWS config: %w", err)
	}

	svc := marketplacecatalog.NewFromConfig(cfg)

	entityID, _, err := findProduct(svc, productName)
	if err != nil {
		return err
	}

	details, err := describeProduct(svc, entityID)
	if err != nil {
		return err
	}

	// Auto-detect base version if not specified
	if baseVersion == "" {
		baseVersion, err = latestVersion(details)
		if err != nil {
			return fmt.Errorf("could not auto-detect base version: %w", err)
		}
		fmt.Printf("Auto-detected base version: %s\n", baseVersion)
	}

	// Dump the base version YAML
	for _, version := range details.Versions {
		if version.VersionTitle == baseVersion {
			data, err := yaml.Marshal(version)
			if err != nil {
				return fmt.Errorf("failed to marshal base version: %w", err)
			}
			filePath := getYamlFilePath(productName, "versions", baseVersion)
			if err := os.WriteFile(filePath, data, 0644); err != nil {
				return fmt.Errorf("failed to write base version YAML: %w", err)
			}
			fmt.Printf("Base version written to %s\n", filePath)
			break
		}
	}

	// Clone base version to new version
	if err := cloneProductVersion(productName, baseVersion, newVersion); err != nil {
		return fmt.Errorf("failed to clone version: %w", err)
	}

	// Update the cloned version with new image and release notes
	if err := updateVersionYAML(productName, newVersion, image, releaseNotes); err != nil {
		return fmt.Errorf("failed to update version YAML: %w", err)
	}

	// Push the new version
	return pushNewVersion(productName, noOp, newVersion)
}

func updateVersionYAML(productName, version, image, releaseNotes string) error {
	filePath := getYamlFilePath(productName, "versions", version)

	data, err := getYAMLData(filePath)
	if err != nil {
		return fmt.Errorf("failed to read version YAML: %w", err)
	}

	data.Releasenotes = releaseNotes
	data.Versiontitle = version

	// Replace all image URIs in all sources
	for i := range data.Sources {
		for j := range data.Sources[i].Images {
			data.Sources[i].Images[j] = image
		}
	}

	yamlBytes, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal updated YAML: %w", err)
	}

	if err := os.WriteFile(filePath, yamlBytes, 0644); err != nil {
		return fmt.Errorf("failed to write updated YAML: %w", err)
	}

	fmt.Printf("Updated version YAML at %s\n", filePath)
	return nil
}
