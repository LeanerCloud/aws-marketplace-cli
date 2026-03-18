package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog/types"
	"gopkg.in/yaml.v2"
)

// YAMLVersionData is the source data structure read from version YAML files.
type YAMLVersionData struct {
	ID                  string            `json:"id"`
	Releasenotes        string            `json:"releasenotes"`
	Upgradeinstructions string            `json:"upgradeinstructions"`
	Versiontitle        string            `json:"versiontitle"`
	Creationdate        time.Time         `json:"creationdate"`
	Sources             []Sources         `json:"sources"`
	Deliveryoptions     []Deliveryoptions `json:"deliveryoptions"`
}

type PlatformCompatibility struct {
	Platform string `json:"platform"`
}

type Sources struct {
	Type          string                `json:"type"`
	ID            string                `json:"id"`
	Images        []string              `json:"images"`
	Compatibility ServicesCompatibility `json:"compatibility"`
}

type ServicesCompatibility struct {
	Awsservices []string `json:"awsservices"`
}

type Instructions struct {
	Usage string `json:"usage"`
}

type Deploymentresources struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

type Recommendations struct {
	Deploymentresources []Deploymentresources `json:"deploymentresources"`
}

type Deliveryoptions struct {
	ID               string                `json:"id"`
	Type             string                `json:"type"`
	Sourceid         string                `json:"sourceid"`
	Title            string                `json:"title"`
	Shortdescription string                `json:"shortdescription"`
	Isrecommended    bool                  `json:"isrecommended"`
	Compatibility    ServicesCompatibility `json:"compatibility"`
	Instructions     Instructions          `json:"instructions"`
	Recommendations  Recommendations       `json:"recommendations"`
	Visibility       string                `json:"visibility"`
}

// DstVersionData is the destination data structure for the AWS Marketplace API.
type DstVersionData struct {
	Version         Version           `json:"Version"`
	DeliveryOptions []DeliveryOptions `json:"DeliveryOptions"`
}

type Version struct {
	ReleaseNotes string `json:"ReleaseNotes"`
	VersionTitle string `json:"VersionTitle"`
}

type DeploymentResources struct {
	Name string `json:"Name"`
	URL  string `json:"Url"`
}

type EcrDeliveryOptionDetails struct {
	DeploymentResources []DeploymentResources `json:"DeploymentResources"`
	CompatibleServices  []string              `json:"CompatibleServices"`
	ContainerImages     []string              `json:"ContainerImages"`
	Description         string                `json:"Description"`
	UsageInstructions   string                `json:"UsageInstructions"`
}

type Details struct {
	EcrDeliveryOptionDetails EcrDeliveryOptionDetails `json:"EcrDeliveryOptionDetails"`
}

type DeliveryOptions struct {
	Details             Details `json:"Details"`
	DeliveryOptionTitle string  `json:"DeliveryOptionTitle"`
}

// convertDeliveryOption maps a single YAML delivery option to the AWS API format.
func convertDeliveryOption(opt Deliveryoptions, images []string) DeliveryOptions {
	resources := make([]DeploymentResources, 0, len(opt.Recommendations.Deploymentresources))
	for _, dr := range opt.Recommendations.Deploymentresources {
		resources = append(resources, DeploymentResources{Name: dr.Text, URL: dr.URL})
	}
	return DeliveryOptions{
		DeliveryOptionTitle: opt.Title,
		Details: Details{
			EcrDeliveryOptionDetails: EcrDeliveryOptionDetails{
				Description:         opt.Shortdescription,
				UsageInstructions:   opt.Instructions.Usage,
				ContainerImages:     images,
				CompatibleServices:  opt.Compatibility.Awsservices,
				DeploymentResources: resources,
			},
		},
	}
}

func (src YAMLVersionData) convertToDst() DstVersionData {
	var images []string
	if len(src.Sources) > 0 {
		images = src.Sources[0].Images
	}

	opts := make([]DeliveryOptions, 0, len(src.Deliveryoptions))
	for i := range src.Deliveryoptions {
		opts = append(opts, convertDeliveryOption(src.Deliveryoptions[i], images))
	}

	return DstVersionData{
		Version: Version{
			ReleaseNotes: src.Releasenotes,
			VersionTitle: src.Versiontitle,
		},
		DeliveryOptions: opts,
	}
}

func getYAMLData(fileName string) (*YAMLVersionData, error) {
	yamlFile, err := os.ReadFile(fileName) //nolint:gosec // G304: path is constructed internally from product/version names, not raw user input
	if err != nil {
		return nil, err
	}
	var data YAMLVersionData
	if err := yaml.Unmarshal(yamlFile, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func dumpVersionsWithClient(svc marketplaceClient, productName string) error {
	entityID, _, err := findProduct(svc, productName)
	if err != nil {
		return err
	}

	details, err := describeProduct(svc, entityID)
	if err != nil {
		return err
	}

	for i := range details.Versions {
		version := &details.Versions[i]
		fileName, err := getYamlFilePath(productName, "versions", version.VersionTitle)
		if err != nil {
			return err
		}
		data, err := yaml.Marshal(version)
		if err != nil {
			return err
		}
		if err := writeFileIfChanged(fileName, data,
			"Data for entity "+entityID+" version "+version.VersionTitle+" has not changed",
			"Data written to "+fileName,
		); err != nil {
			return err
		}
	}
	return nil
}

func dumpVersions(productName string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	return dumpVersionsWithClient(marketplacecatalog.NewFromConfig(cfg), productName)
}

func pushNewVersionWithClient(svc marketplaceClient, productName string, noOp bool, version string) error {
	entityID, foundType, err := findProduct(svc, productName)
	if err != nil {
		return err
	}

	versionPath, err := getYamlFilePath(productName, "versions", version)
	if err != nil {
		return err
	}
	srcVersionDetails, err := getYAMLData(versionPath)
	if err != nil {
		return errors.New("could not read version details: " + err.Error())
	}

	dstVersionDetails := srcVersionDetails.convertToDst()

	if noOp {
		changeSetJSON, _ := json.MarshalIndent(dstVersionDetails, "", "  ")
		fmt.Println(string(changeSetJSON))
		return nil
	}

	versionBytes, err := json.Marshal(dstVersionDetails)
	if err != nil {
		return err
	}

	entityTypeIdentifier, _ := getEntityTypeAndChangeType(foundType)
	versionChangeType := "AddDeliveryOptions"
	if foundType == productTypeServer {
		versionChangeType = "CreateVersion"
	}

	changeSetInput := &marketplacecatalog.StartChangeSetInput{
		Catalog: aws.String("AWSMarketplace"),
		ChangeSet: []types.Change{
			{
				ChangeType: aws.String(versionChangeType),
				ChangeName: aws.String("AddNewVersion"),
				Entity: &types.Entity{
					Type:       aws.String(entityTypeIdentifier),
					Identifier: aws.String(entityID),
				},
				Details: aws.String(string(versionBytes)),
			},
		},
		ChangeSetName: aws.String(fmt.Sprintf("Push %s version %s", productName, version)),
	}

	_, err = svc.StartChangeSet(context.Background(), changeSetInput)
	if err != nil {
		return errors.New("could not start change set: " + err.Error())
	}

	fmt.Printf("Changeset created for product %s (%s) with entity ID %s\n", productName, foundType, entityID)
	return nil
}

func pushNewVersion(productName string, noOp bool, version string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return errors.New("couldn't load default config")
	}
	return pushNewVersionWithClient(marketplacecatalog.NewFromConfig(cfg), productName, noOp, version)
}

func cloneProductVersion(productName, srcVersion, dstVersion string) error {
	srcFilePath, err := getYamlFilePath(productName, "versions", srcVersion)
	if err != nil {
		return err
	}
	dstFilePath, err := getYamlFilePath(productName, "versions", dstVersion)
	if err != nil {
		return err
	}

	existingData, err := os.ReadFile(dstFilePath) //nolint:gosec // G304: path is constructed internally, not from raw user input
	if err == nil {
		srcData, err := os.ReadFile(srcFilePath) //nolint:gosec // G304: path is constructed internally, not from raw user input
		if err != nil {
			return fmt.Errorf("failed to read source file: %w", err)
		}
		if bytes.Equal(existingData, srcData) {
			fmt.Printf("Data for product %s version %s has not changed\n", productName, srcVersion)
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read destination file: %w", err)
	}

	input, err := os.ReadFile(srcFilePath) //nolint:gosec // G304: path is constructed internally, not from raw user input
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}
	output := bytes.ReplaceAll(input, []byte(srcVersion), []byte(dstVersion))
	if err := os.WriteFile(dstFilePath, output, 0o644); err != nil { //nolint:gosec // G306: 0644 is intentional — user-readable YAML version files
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	fmt.Printf("Data written to %s\n", dstFilePath)
	return nil
}
