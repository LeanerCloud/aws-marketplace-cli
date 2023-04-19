package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog/types"
	"gopkg.in/yaml.v2"
)

//source data structure read from the version YAML file

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

// destination data structure

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

func (src YAMLVersionData) convertToDst() DstVersionData {
	var dst DstVersionData
	var deliveryOptions []DeliveryOptions
	var version Version

	// set version fields
	version.ReleaseNotes = src.Releasenotes
	version.VersionTitle = src.Versiontitle

	// set delivery options fields
	for _, deliveryOption := range src.Deliveryoptions {
		var d DeliveryOptions
		var details Details

		d.DeliveryOptionTitle = deliveryOption.Title

		// set EcrDeliveryOptionDetails fields
		details.EcrDeliveryOptionDetails.Description = deliveryOption.Shortdescription
		details.EcrDeliveryOptionDetails.UsageInstructions = deliveryOption.Instructions.Usage
		details.EcrDeliveryOptionDetails.ContainerImages = src.Sources[0].Images
		details.EcrDeliveryOptionDetails.CompatibleServices = deliveryOption.Compatibility.Awsservices

		for _, deploymentResource := range deliveryOption.Recommendations.Deploymentresources {
			details.EcrDeliveryOptionDetails.DeploymentResources = append(details.EcrDeliveryOptionDetails.DeploymentResources, DeploymentResources{
				Name: deploymentResource.Text,
				URL:  deploymentResource.URL,
			})
		}

		d.Details = details

		deliveryOptions = append(deliveryOptions, d)
	}

	dst.Version = version
	dst.DeliveryOptions = deliveryOptions

	return dst
}

func getYAMLData(fileName string) (*YAMLVersionData, error) {
	yamlFile, err := ioutil.ReadFile(fileName)
	if err != nil {
		return nil, err
	}

	var data YAMLVersionData
	if err := yaml.Unmarshal(yamlFile, &data); err != nil {
		return nil, err
	}

	return &data, nil
}

func dumpVersions(productName string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}

	svc := marketplacecatalog.NewFromConfig(cfg)

	entityID, err := getProductEntityID(svc, &productName)
	if err != nil {
		return err
	}

	resp, err := svc.DescribeEntity(context.Background(), &marketplacecatalog.DescribeEntityInput{
		EntityId: entityID,
		Catalog:  aws.String("AWSMarketplace"),
	})
	if err != nil {
		return err
	}

	var details EntityDetails
	if err := json.Unmarshal([]byte(*resp.Details), &details); err != nil {
		return err
	}

	for _, version := range details.Versions {

		fileName := getYamlFilePath(productName, "versions", version.VersionTitle)
		data, err := yaml.Marshal(version)
		if err != nil {
			return err
		}

		// Check if file has changed before writing to it
		if _, err := os.Stat(fileName); err == nil {
			existingData, err := ioutil.ReadFile(fileName)
			if err != nil {
				return err
			}
			if bytes.Equal(existingData, data) {
				fmt.Printf("Data for entity %s has not changed %s \n", *entityID, data)
				return nil
			}
		}

		if err := ioutil.WriteFile(fileName, data, 0644); err != nil {
			return err
		}

		fmt.Printf("Data written to %s\n", fileName)
	}
	return nil
}
func pushNewVersion(productName string, noOp bool, version string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return errors.New("couldn't load default config")
	}

	svc := marketplacecatalog.NewFromConfig(cfg)

	entityID, err := getProductEntityID(svc, &productName)
	if err != nil {
		return err
	}

	srcVersionDetails, err := getYAMLData(getYamlFilePath(productName, "versions", version))

	if err != nil {
		return errors.New("could not read version details: " + err.Error())
	}

	dstVersionDetails := srcVersionDetails.convertToDst()

	versionBytes, err := json.Marshal(dstVersionDetails)
	if err != nil {
		return err
	}

	if noOp {
		changeSetJSON, _ := json.MarshalIndent(dstVersionDetails, "", "  ")
		fmt.Println(string(changeSetJSON))
		return nil
	}

	// Create a changeset to update the product
	change := types.Change{
		ChangeType: aws.String("AddDeliveryOptions"),
		ChangeName: aws.String("AddNewVersion"),
		Entity: &types.Entity{
			Type:       aws.String("ContainerProduct@1.0"),
			Identifier: entityID,
		},
		Details: aws.String(string(versionBytes)),
	}
	changeSetInput := &marketplacecatalog.StartChangeSetInput{
		Catalog: aws.String("AWSMarketplace"),
		ChangeSet: []types.Change{
			change,
		},
		ChangeSetName: aws.String(fmt.Sprintf("Push % version %s", productName, version)),
	}

	if noOp {
		changeSetJSON, _ := json.MarshalIndent(changeSetInput, "", "  ")
		fmt.Println(string(changeSetJSON))
		return nil
	}

	_, err = svc.StartChangeSet(context.Background(), changeSetInput)
	if err != nil {
		return errors.New("could not start change set: " + err.Error())
	}

	fmt.Printf("Changeset created for product %s with entity ID %s\n", productName, *entityID)
	return nil
}

func cloneProductVersion(productName, srcVersion, dstVersion string) error {
	srcFilePath := getYamlFilePath(productName, "versions", srcVersion)
	dstFilePath := getYamlFilePath(productName, "versions", dstVersion)

	existingData, err := ioutil.ReadFile(dstFilePath)
	if err == nil {
		srcData, err := ioutil.ReadFile(srcFilePath)
		if err != nil {
			return fmt.Errorf("failed to read source file: %v", err)
		}
		if bytes.Equal(existingData, srcData) {
			fmt.Printf("Data for product %s version %s has not changed\n", productName, srcVersion)
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read destination file: %v", err)
	}

	input, err := ioutil.ReadFile(srcFilePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %v", err)
	}
	// Replace srcVersion with dstVersion inside the YAML content
	output := bytes.Replace(input, []byte(srcVersion), []byte(dstVersion), -1)
	err = ioutil.WriteFile(dstFilePath, output, 0644)
	if err != nil {
		return fmt.Errorf("failed to write destination file: %v", err)
	}

	fmt.Printf("Data written to %s\n", dstFilePath)
	return nil
}
