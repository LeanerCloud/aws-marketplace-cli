package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog/types"
	"gopkg.in/yaml.v2"
)

type EntityDetails struct {
	Versions []struct {
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
	} `json:"Versions"`
	Description struct {
		Highlights      []string `json:"Highlights"`
		LongDescription string   `json:"LongDescription"`
		// ProductCode        string   `json:"ProductCode"`
		// Manufacturer       any      `json:"Manufacturer"`
		// ProductState string `json:"ProductState"`
		// Visibility   string `json:"Visibility"`
		// AssociatedProducts any      `json:"AssociatedProducts"`
		Sku              any      `json:"Sku"`
		SearchKeywords   []string `json:"SearchKeywords"`
		ProductTitle     string   `json:"ProductTitle"`
		ShortDescription string   `json:"ShortDescription"`
		Categories       []string `json:"Categories"`
	} `json:"Description"`
	Targeting struct {
		PositiveTargeting struct {
			BuyerAccounts []string `json:"BuyerAccounts"`
		} `json:"PositiveTargeting"`
	} `json:"Targeting"`
	PromotionalResources struct {
		PromotionalMedia    any    `json:"PromotionalMedia"`
		LogoURL             string `json:"LogoUrl"`
		AdditionalResources []struct {
			Type string `json:"Type"`
			Text string `json:"Text"`
			URL  string `json:"Url"`
		} `json:"AdditionalResources"`
		Videos []struct {
			Type  string `json:"Type"`
			Title string `json:"Title"`
			URL   string `json:"Url"`
		} `json:"Videos"`
	} `json:"PromotionalResources"`
	Dimensions []struct {
		Types       []string `json:"Types"`
		Description string   `json:"Description"`
		Unit        string   `json:"Unit"`
		Key         string   `json:"Key"`
		Name        string   `json:"Name"`
	} `json:"Dimensions"`
	SupportInformation struct {
		Description string `json:"Description"`
		Resources   []any  `json:"Resources"`
	} `json:"SupportInformation"`
	RegionAvailability struct {
		Restrict            []any    `json:"Restrict"`
		Regions             []string `json:"Regions"`
		FutureRegionSupport any      `json:"FutureRegionSupport"`
	} `json:"RegionAvailability"`
	Repositories []struct {
		URL  string `json:"Url"`
		Type string `json:"Type"`
	} `json:"Repositories"`
}

func listProducts(productType string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}

	svc := marketplacecatalog.NewFromConfig(cfg)
	params := &marketplacecatalog.ListEntitiesInput{
		Catalog:    aws.String("AWSMarketplace"),
		EntityType: aws.String(productType),
		MaxResults: aws.Int32(10),
	}
	for {
		resp, err := svc.ListEntities(context.Background(), params)
		if err != nil {
			return err
		}
		for _, entity := range resp.EntitySummaryList {
			fmt.Printf("%s\n", *entity.Name)
		}
		if resp.NextToken == nil {
			break
		}
		params.NextToken = resp.NextToken
	}
	return nil
}

func getProductEntityID(svc *marketplacecatalog.Client, productName *string) (*string, error) {
	input := &marketplacecatalog.ListEntitiesInput{
		Catalog:    aws.String("AWSMarketplace"),
		EntityType: aws.String("ContainerProduct"),
	}
	res, err := svc.ListEntities(context.Background(), input)
	if err != nil {
		return nil, err
	}
	if len(res.EntitySummaryList) == 0 {
		return nil, fmt.Errorf("could not find entity ID for product %s", *productName)
	}
	for _, entity := range res.EntitySummaryList {
		if *entity.Name == *productName {
			return entity.EntityId, nil
		}
	}
	return nil, errors.New("entity not found")
}
func getYamlFilePath(productName, subdir, fileName string) string {
	dirName := fmt.Sprintf("data/%s/%s", productName, subdir)
	os.MkdirAll(dirName, 0755)
	return filepath.Join(dirName, fileName+".yaml")
}

func dumpProduct(productName string) error {
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

	fmt.Printf("DescribeEntity details: #######%s########", *resp.Details)

	var details EntityDetails
	if err := json.Unmarshal([]byte(*resp.Details), &details); err != nil {
		return err
	}

	details.Versions = nil

	fileName := getYamlFilePath(productName, "", "description")
	data, err := yaml.Marshal(details)
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
	return nil
}

func updateProduct(productName string, noOp bool) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return errors.New("couldn't load default config")
	}

	svc := marketplacecatalog.NewFromConfig(cfg)

	entityID, err := getProductEntityID(svc, &productName)

	if err != nil {
		return err
	}

	// Read the YAML file
	data, err := ioutil.ReadFile(getYamlFilePath(productName, "", "description"))
	if err != nil {
		return err
	}
	var details EntityDetails
	if err := yaml.Unmarshal(data, &details); err != nil {
		return err
	}

	detailsBytes, err := json.Marshal(details.Description)
	if err != nil {
		return err
	}

	// Create a changeset to update the product
	change := types.Change{
		ChangeType: aws.String("UpdateInformation"),
		ChangeName: aws.String("UpdateProductInformation"),
		Entity: &types.Entity{
			Type:       aws.String("ContainerProduct@1.0"),
			Identifier: entityID,
		},
		Details: aws.String(string(detailsBytes)),
	}
	changeSetInput := &marketplacecatalog.StartChangeSetInput{
		Catalog: aws.String("AWSMarketplace"),
		ChangeSet: []types.Change{
			change,
		},
		ChangeSetName: aws.String("Updated product Information for " + productName),
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
