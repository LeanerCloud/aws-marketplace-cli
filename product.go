package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog/types"
	"gopkg.in/yaml.v2"
)

// marketplaceClient abstracts the AWS Marketplace Catalog API for testability.
type marketplaceClient interface {
	ListEntities(ctx context.Context, params *marketplacecatalog.ListEntitiesInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.ListEntitiesOutput, error)
	DescribeEntity(ctx context.Context, params *marketplacecatalog.DescribeEntityInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.DescribeEntityOutput, error)
	StartChangeSet(ctx context.Context, params *marketplacecatalog.StartChangeSetInput, optFns ...func(*marketplacecatalog.Options)) (*marketplacecatalog.StartChangeSetOutput, error)
}

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
		Highlights       []string `json:"Highlights"`
		LongDescription  string   `json:"LongDescription"`
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

var allProductTypes = []string{
	"ServerProduct",
	"ContainerProduct",
	"DataProduct",
	"MachinelearningProduct",
	"SaaSProduct",
	"ServiceProduct",
	"SolutionProduct",
	"SupportProduct",
}

// entityTypeVersionMap maps product types to their versioned AWS entity type identifiers.
var entityTypeVersionMap = map[string]string{
	"ServerProduct":          "ServerProduct@1.0",
	"ContainerProduct":       "ContainerProduct@1.0",
	"DataProduct":            "DataProduct@1.0",
	"MachinelearningProduct": "MachinelearningProduct@1.0",
	"SaaSProduct":            "SaaSProduct@1.0",
	"ServiceProduct":         "ServiceProduct@1.0",
	"SolutionProduct":        "SolutionProduct@1.0",
	"SupportProduct":         "SupportProduct@1.0",
}

func getEntityTypeAndChangeType(productType string) (string, string) {
	identifier, ok := entityTypeVersionMap[productType]
	if !ok {
		identifier = productType + "@1.0"
	}
	return identifier, "UpdateInformation"
}

// resolveProductTypes validates requestedType and returns the list to query.
// "all" returns all supported types; otherwise validates the single requested type.
func resolveProductTypes(requestedType string) ([]string, error) {
	if requestedType == "all" {
		return allProductTypes, nil
	}
	if !slices.Contains(allProductTypes, requestedType) {
		return nil, fmt.Errorf("invalid product type: %s. Valid types are: %s, or use 'all' to list all types",
			requestedType, strings.Join(allProductTypes, ", "))
	}
	return []string{requestedType}, nil
}

// paginateEntityNames fetches all entity names for the given params, following pagination tokens.
func paginateEntityNames(ctx context.Context, svc marketplaceClient, params *marketplacecatalog.ListEntitiesInput) ([]string, error) {
	resp, err := svc.ListEntities(ctx, params)
	if err != nil {
		return nil, err
	}
	var names []string
	for {
		for _, entity := range resp.EntitySummaryList {
			names = append(names, *entity.Name)
		}
		if resp.NextToken == nil {
			break
		}
		params.NextToken = resp.NextToken
		resp, err = svc.ListEntities(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("pagination: %v", err)
		}
	}
	return names, nil
}

// collectProductNames fetches all product names for a single type, suppressing invalid-entity-type errors.
func collectProductNames(ctx context.Context, svc marketplaceClient, productType string) ([]string, error) {
	params := &marketplacecatalog.ListEntitiesInput{
		Catalog:    aws.String("AWSMarketplace"),
		EntityType: aws.String(productType),
		MaxResults: aws.Int32(50),
	}
	names, err := paginateEntityNames(ctx, svc, params)
	if err != nil {
		var ve *types.ValidationException
		if errors.As(err, &ve) && strings.Contains(err.Error(), "entity type") {
			return nil, nil
		}
		return nil, fmt.Errorf("error listing %s: %v", productType, err)
	}
	return names, nil
}

func printProductType(productType string, names []string) {
	fmt.Printf("\n%s (%d products):\n", productType, len(names))
	for _, name := range names {
		fmt.Printf("  - %s\n", name)
	}
}

func printNotFound(requestedType string) {
	if requestedType == "all" {
		fmt.Println("No products found in any category")
	} else {
		fmt.Printf("No products found of type: %s\n", requestedType)
	}
}

func listProductsWithClient(svc marketplaceClient, requestedType string) error {
	productTypes, err := resolveProductTypes(requestedType)
	if err != nil {
		return err
	}

	ctx := context.Background()
	foundAny := false

	for _, productType := range productTypes {
		names, err := collectProductNames(ctx, svc, productType)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			continue
		}
		foundAny = true
		sort.Strings(names)
		printProductType(productType, names)
	}

	if !foundAny {
		printNotFound(requestedType)
	}
	return nil
}

func listProducts(requestedType string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	return listProductsWithClient(marketplacecatalog.NewFromConfig(cfg), requestedType)
}

func getProductEntityID(svc marketplaceClient, productName *string, productType string) (*string, error) {
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

func findProduct(svc marketplaceClient, productName string) (entityID, productType string, err error) {
	var lastErr error
	for _, pt := range allProductTypes {
		eid, e := getProductEntityID(svc, &productName, pt)
		if e == nil {
			return *eid, pt, nil
		}
		lastErr = e
	}
	return "", "", fmt.Errorf("could not find product %s in any supported type: %v", productName, lastErr)
}

func describeProduct(svc marketplaceClient, entityID string) (*EntityDetails, error) {
	resp, err := svc.DescribeEntity(context.Background(), &marketplacecatalog.DescribeEntityInput{
		EntityId: aws.String(entityID),
		Catalog:  aws.String("AWSMarketplace"),
	})
	if err != nil {
		return nil, err
	}
	var details EntityDetails
	if err := json.Unmarshal([]byte(*resp.Details), &details); err != nil {
		return nil, err
	}
	return &details, nil
}

func latestVersion(details *EntityDetails) (string, error) {
	if len(details.Versions) == 0 {
		return "", errors.New("product has no versions")
	}
	latest := details.Versions[0]
	for i := range details.Versions[1:] {
		if details.Versions[i+1].CreationDate.After(latest.CreationDate) {
			latest = details.Versions[i+1]
		}
	}
	return latest.VersionTitle, nil
}

func getYamlFilePath(productName, subdir, fileName string) (string, error) {
	dirName := fmt.Sprintf("data/%s/%s", productName, subdir)
	if err := os.MkdirAll(dirName, 0o755); err != nil { //nolint:gosec // G301: 0755 is intentional for user-owned data directories
		return "", fmt.Errorf("failed to create directory %s: %w", dirName, err)
	}
	return filepath.Join(dirName, fileName+".yaml"), nil
}

// writeFileIfChanged writes data to fileName only if the content differs from what is already on disk.
func writeFileIfChanged(fileName string, data []byte, unchangedMsg, writtenMsg string) error {
	if _, err := os.Stat(fileName); err == nil {
		existing, err := os.ReadFile(fileName) //nolint:gosec // G304: path is constructed internally from product/version names, not from raw user input
		if err != nil {
			return err
		}
		if bytes.Equal(existing, data) {
			fmt.Println(unchangedMsg)
			return nil
		}
	}
	if err := os.WriteFile(fileName, data, 0o644); err != nil { //nolint:gosec // G306: 0644 is intentional — these are user-readable YAML config files
		return err
	}
	fmt.Println(writtenMsg)
	return nil
}

func dumpProductWithClient(svc marketplaceClient, productName string) error {
	entityID, _, err := findProduct(svc, productName)
	if err != nil {
		return err
	}

	details, err := describeProduct(svc, entityID)
	if err != nil {
		return err
	}
	details.Versions = nil

	fileName, err := getYamlFilePath(productName, "", "description")
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(details)
	if err != nil {
		return err
	}

	return writeFileIfChanged(fileName, data,
		"Data for entity "+entityID+" has not changed",
		"Data written to "+fileName)
}

func dumpProduct(productName string) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return err
	}
	return dumpProductWithClient(marketplacecatalog.NewFromConfig(cfg), productName)
}

func updateProductWithClient(svc marketplaceClient, productName string, noOp bool) error {
	entityID, foundType, err := findProduct(svc, productName)
	if err != nil {
		return err
	}

	descPath, err := getYamlFilePath(productName, "", "description")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(descPath) //nolint:gosec // G304: path is constructed internally from product name, not raw user input
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
	changeSetInput := &marketplacecatalog.StartChangeSetInput{
		Catalog: aws.String("AWSMarketplace"),
		ChangeSet: []types.Change{
			{
				ChangeType: aws.String(changeType),
				ChangeName: aws.String("UpdateProductInformation"),
				Entity: &types.Entity{
					Type:       aws.String(entityTypeIdentifier),
					Identifier: aws.String(entityID),
				},
				Details: aws.String(string(detailsBytes)),
			},
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

	fmt.Printf("Changeset created for product %s (%s) with entity ID %s\n", productName, foundType, entityID)
	return nil
}

func updateProduct(productName string, noOp bool) error {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return errors.New("couldn't load default config")
	}
	return updateProductWithClient(marketplacecatalog.NewFromConfig(cfg), productName, noOp)
}
