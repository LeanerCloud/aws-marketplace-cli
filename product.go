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
	"sort"
	"strings"
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

func listProducts(requestedType string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}

	svc := marketplacecatalog.NewFromConfig(cfg)

	// Define all valid product types
	productTypes := []string{
		"ServerProduct",
		"ContainerProduct",
		"DataProduct",
		"MachinelearningProduct",
		"SaaSProduct",
		"ServiceProduct",
		"SolutionProduct",
		"SupportProduct",
	}

	// If a specific type is requested, only list that type
	if requestedType != "all" {
		found := false
		for _, validType := range productTypes {
			if validType == requestedType {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid product type: %s. Valid types are: %s, or use 'all' to list all types",
				requestedType, strings.Join(productTypes, ", "))
		}
		productTypes = []string{requestedType}
	}

	// Track if we found any products
	foundAny := false

	// List products for each type
	for _, productType := range productTypes {
		params := &marketplacecatalog.ListEntitiesInput{
			Catalog:    aws.String("AWSMarketplace"),
			EntityType: aws.String(productType),
			MaxResults: aws.Int32(50),
		}

		resp, err := svc.ListEntities(context.Background(), params)
		if err != nil {
			// Only skip if it's a validation error specifically about invalid entity type
			var ve *types.ValidationException
			if errors.As(err, &ve) && strings.Contains(err.Error(), "entity type") {
				continue
			}
			return fmt.Errorf("error listing %s: %v", productType, err)
		}

		products := make([]string, 0)
		for {
			for _, entity := range resp.EntitySummaryList {
				products = append(products, *entity.Name)
			}

			if resp.NextToken == nil {
				break
			}

			params.NextToken = resp.NextToken
			resp, err = svc.ListEntities(context.Background(), params)
			if err != nil {
				return fmt.Errorf("error listing %s (pagination): %v", productType, err)
			}
		}

		// Only print the type if we found products of this type
		if len(products) > 0 {
			foundAny = true
			fmt.Printf("\n%s (%d products):\n", productType, len(products))
			sort.Strings(products)
			for _, name := range products {
				fmt.Printf("  - %s\n", name)
			}
		}
	}

	if !foundAny {
		if requestedType == "all" {
			fmt.Println("No products found in any category")
		} else {
			fmt.Printf("No products found of type: %s\n", requestedType)
		}
	}

	return nil
}

func getProductEntityID(svc *marketplacecatalog.Client, productName *string, productType string) (*string, error) {
	input := &marketplacecatalog.ListEntitiesInput{
		Catalog:    aws.String("AWSMarketplace"),
		EntityType: aws.String(productType),
	}
	res, err := svc.ListEntities(context.Background(), input)
	if err != nil {
		return nil, err
	}
	if len(res.EntitySummaryList) == 0 {
		return nil, fmt.Errorf("could not find entity ID for product %s of type %s", *productName, productType)
	}
	for _, entity := range res.EntitySummaryList {
		if *entity.Name == *productName {
			return entity.EntityId, nil
		}
	}
	return nil, fmt.Errorf("entity not found: %s of type %s", *productName, productType)
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

	// Try different product types in order of likelihood
	productTypes := []string{
		"ServerProduct",
		"ContainerProduct",
		"DataProduct",
		"MachinelearningProduct",
		"SaaSProduct",
		"ServiceProduct",
		"SolutionProduct",
		"SupportProduct",
	}
	var entityID *string
	var lastErr error

	for _, productType := range productTypes {
		entityID, lastErr = getProductEntityID(svc, &productName, productType)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		return fmt.Errorf("could not find product %s in any supported type: %v", productName, lastErr)
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
			fmt.Printf("Data for entity %s has not changed\n", *entityID)
			return nil
		}
	}

	if err := ioutil.WriteFile(fileName, data, 0644); err != nil {
		return err
	}

	fmt.Printf("Data written to %s\n", fileName)
	return nil
}

// Helper function to get correct type identifiers
func getEntityTypeAndChangeType(productType string) (string, string) {
	switch productType {
	case "ServerProduct":
		return "ServerProduct@1.0", "UpdateInformation"
	case "ContainerProduct":
		return "ContainerProduct@1.0", "UpdateInformation"
	case "DataProduct":
		return "DataProduct@1.0", "UpdateInformation"
	case "MachinelearningProduct":
		return "MachinelearningProduct@1.0", "UpdateInformation"
	case "SaaSProduct":
		return "SaaSProduct@1.0", "UpdateInformation"
	case "ServiceProduct":
		return "ServiceProduct@1.0", "UpdateInformation"
	case "SolutionProduct":
		return "SolutionProduct@1.0", "UpdateInformation"
	case "SupportProduct":
		return "SupportProduct@1.0", "UpdateInformation"
	default:
		return productType + "@1.0", "UpdateInformation"
	}
}

func updateProduct(productName string, noOp bool) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return errors.New("couldn't load default config")
	}

	svc := marketplacecatalog.NewFromConfig(cfg)

	productTypes := []string{
		"ServerProduct",
		"ContainerProduct",
		"DataProduct",
		"MachinelearningProduct",
		"SaaSProduct",
		"ServiceProduct",
		"SolutionProduct",
		"SupportProduct",
	}

	var entityID *string
	var lastErr error
	var foundType string

	for _, productType := range productTypes {
		entityID, lastErr = getProductEntityID(svc, &productName, productType)
		if lastErr == nil {
			foundType = productType
			break
		}
	}

	if lastErr != nil {
		return fmt.Errorf("could not find product %s in any supported type: %v", productName, lastErr)
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

	entityTypeIdentifier, changeType := getEntityTypeAndChangeType(foundType)

	// Create a changeset to update the product
	change := types.Change{
		ChangeType: aws.String(changeType),
		ChangeName: aws.String("UpdateProductInformation"),
		Entity: &types.Entity{
			Type:       aws.String(entityTypeIdentifier),
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

	fmt.Printf("Changeset created for product %s (%s) with entity ID %s\n", productName, foundType, *entityID)
	return nil
}
